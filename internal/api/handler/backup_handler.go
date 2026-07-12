package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	appbackup "telegram-message-forward/internal/app/backup"
)

type BackupHandler struct{ svc *appbackup.Service }

func NewBackupHandler(svc *appbackup.Service) *BackupHandler { return &BackupHandler{svc: svc} }

type backupExportRequest struct {
	Password        string `json:"password" binding:"required"`
	IncludeSessions bool   `json:"include_sessions"`
}

func (h *BackupHandler) Export(c *gin.Context) {
	var req backupExportRequest
	if !bindJSON(c, &req) {
		return
	}
	archive, manifest, err := h.svc.Export(c.Request.Context(), req.Password, req.IncludeSessions)
	if err != nil {
		h.respond(c, err)
		return
	}
	name := fmt.Sprintf("tmf-config-%s.tmfbackup", manifest.CreatedAt.Format("20060102-150405"))
	c.Header("Content-Disposition", `attachment; filename="`+name+`"`)
	c.Data(http.StatusOK, "application/vnd.tmf.config-backup+json", archive)
}

func (h *BackupHandler) Inspect(c *gin.Context) {
	archive, password, ok := readBackupForm(c)
	if !ok {
		return
	}
	preview, err := h.svc.Inspect(c.Request.Context(), archive, password)
	if err != nil {
		h.respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": preview})
}

func (h *BackupHandler) Restore(c *gin.Context) {
	archive, password, ok := readBackupForm(c)
	if !ok {
		return
	}
	confirmed, _ := strconv.ParseBool(c.PostForm("confirmed"))
	if !confirmed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "恢复前必须先预览并明确确认"})
		return
	}
	result, err := h.svc.Restore(c.Request.Context(), archive, password)
	if err != nil {
		h.respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func readBackupForm(c *gin.Context) ([]byte, string, bool) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择备份文件"})
		return nil, "", false
	}
	opened, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取备份文件失败"})
		return nil, "", false
	}
	defer opened.Close()
	archive, err := appbackup.ReadArchive(opened)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return nil, "", false
	}
	password := c.PostForm("password")
	if len(password) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": appbackup.ErrWeakPassword.Error()})
		return nil, "", false
	}
	return archive, password, true
}

func (h *BackupHandler) respond(c *gin.Context, err error) {
	if errors.Is(err, appbackup.ErrWeakPassword) || errors.Is(err, appbackup.ErrInvalidArchive) || errors.Is(err, appbackup.ErrArchiveTooLarge) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "time": time.Now().UTC()})
}
