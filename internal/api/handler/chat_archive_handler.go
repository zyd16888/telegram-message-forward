package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"telegram-message-forward/internal/api/dto"
	apparchive "telegram-message-forward/internal/app/chatarchive"
	domainarchive "telegram-message-forward/internal/domain/chatarchive"
)

// ChatArchiveHandler 提供聊天归档与导出接口。
type ChatArchiveHandler struct {
	svc *apparchive.Service
}

// NewChatArchiveHandler 创建聊天归档 handler。
func NewChatArchiveHandler(svc *apparchive.Service) *ChatArchiveHandler {
	return &ChatArchiveHandler{svc: svc}
}

type createChatExportRequest struct {
	AccountID     int64  `json:"account_id" binding:"required"`
	PeerType      string `json:"peer_type" binding:"required"`
	PeerID        int64  `json:"peer_id" binding:"required"`
	FromDate      string `json:"from_date"`
	ToDate        string `json:"to_date"`
	MaxMessages   int    `json:"max_messages"`
	IncludeMedia  bool   `json:"include_media"`
	MediaMaxBytes int64  `json:"media_max_bytes"`
}

// CreateJob POST /chat-exports
func (h *ChatArchiveHandler) CreateJob(c *gin.Context) {
	var req createChatExportRequest
	if !bindJSON(c, &req) {
		return
	}
	from, ok := parseOptionalTimeValue(c, "from_date", req.FromDate)
	if !ok {
		return
	}
	to, ok := parseOptionalTimeValue(c, "to_date", req.ToDate)
	if !ok {
		return
	}

	job, archive, err := h.svc.CreateJob(c.Request.Context(), apparchive.CreateJobInput{
		AccountID: req.AccountID, PeerType: req.PeerType, PeerID: req.PeerID,
		FromDate: from, ToDate: to, MaxMessages: req.MaxMessages,
		IncludeMedia: req.IncludeMedia, MediaMaxBytes: req.MediaMaxBytes,
	})
	if err != nil {
		h.respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"data":    dto.NewChatExportJobDTO(job),
		"archive": dto.NewChatArchiveDTO(archive),
	})
}

// GetJob GET /chat-exports/:id
func (h *ChatArchiveHandler) GetJob(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	job, err := h.svc.GetJob(c.Request.Context(), id)
	if err != nil {
		h.respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewChatExportJobDTO(job)})
}

// ListJobs GET /chat-exports?archive_id=
func (h *ChatArchiveHandler) ListJobs(c *gin.Context) {
	archiveID := parseInt64Default(c.Query("archive_id"), 0)
	jobs, err := h.svc.ListJobs(c.Request.Context(), archiveID, parseIntDefault(c.Query("limit"), 20))
	if err != nil {
		h.respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewChatExportJobDTOs(jobs)})
}

// CancelJob POST /chat-exports/:id/cancel
func (h *ChatArchiveHandler) CancelJob(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.Cancel(c.Request.Context(), id); err != nil {
		h.respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ListArchives GET /chat-archives
func (h *ChatArchiveHandler) ListArchives(c *gin.Context) {
	items, err := h.svc.ListArchives(c.Request.Context())
	if err != nil {
		h.respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewChatArchiveDTOs(items)})
}

// GetArchive GET /chat-archives/:id
func (h *ChatArchiveHandler) GetArchive(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	archive, err := h.svc.GetArchive(c.Request.Context(), id)
	if err != nil {
		h.respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewChatArchiveDTO(archive)})
}

// DeleteArchive DELETE /chat-archives/:id
func (h *ChatArchiveHandler) DeleteArchive(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteArchive(c.Request.Context(), id); err != nil {
		h.respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// SearchMessages GET /chat-archives/:id/messages
func (h *ChatArchiveHandler) SearchMessages(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	from, ok := parseOptionalTime(c, "from")
	if !ok {
		return
	}
	to, ok := parseOptionalTime(c, "to")
	if !ok {
		return
	}
	out, ok := parseOptionalDirection(c)
	if !ok {
		return
	}
	includeService, err := strconv.ParseBool(c.DefaultQuery("include_service", "false"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "include_service 必须为 true 或 false"})
		return
	}

	items, hasMore, err := h.svc.SearchMessages(c.Request.Context(), domainarchive.MessageQuery{
		ArchiveID: id, Keyword: c.Query("q"),
		SenderID: parseInt64Default(c.Query("sender_id"), 0),
		Out:      out, IncludeService: includeService,
		From: from, To: to,
		BeforeID: parseInt64Default(c.Query("before_id"), 0),
		Limit:    parseIntDefault(c.Query("limit"), 50),
	})
	if err != nil {
		h.respond(c, err)
		return
	}
	var nextCursor int64
	if hasMore && len(items) > 0 {
		nextCursor = items[len(items)-1].ID
	}
	c.JSON(http.StatusOK, gin.H{
		"data": dto.NewChatArchiveMessageDTOs(items), "has_more": hasMore, "next_cursor": nextCursor,
	})
}

// Download GET /chat-archives/:id/download?format=jsonl|csv|md
//
// 流式响应：几万条消息的导出有几十 MB，不能先在内存里拼完整个文件。
func (h *ChatArchiveHandler) Download(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	format, err := apparchive.ParseFormat(c.Query("format"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	from, ok := parseOptionalTime(c, "from")
	if !ok {
		return
	}
	to, ok := parseOptionalTime(c, "to")
	if !ok {
		return
	}
	includeService, err := strconv.ParseBool(c.DefaultQuery("include_service", "false"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "include_service 必须为 true 或 false"})
		return
	}

	// 先取归档：既校验存在性，也用于生成文件名。响应头一旦写出就无法再改状态码。
	archive, err := h.svc.GetArchive(c.Request.Context(), id)
	if err != nil {
		h.respond(c, err)
		return
	}

	c.Header("Content-Type", format.ContentType())
	c.Header("Content-Disposition", `attachment; filename="`+apparchive.ExportFileName(archive, format)+`"`)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Status(http.StatusOK)

	if err := h.svc.Render(c.Request.Context(), format, domainarchive.ExportQuery{
		ArchiveID: id, From: from, To: to, IncludeService: includeService,
	}, c.Writer); err != nil {
		// 响应头已写出，无法再改状态码：中断连接让客户端看到不完整下载，
		// 好过静默返回一个被截断但看起来正常的文件。
		_ = c.Error(err)
		c.Abort()
	}
}

func (h *ChatArchiveHandler) respond(c *gin.Context, err error) {
	switch {
	case errors.Is(err, apparchive.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, apparchive.ErrAccountInactive):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, apparchive.ErrJobNotCancellable):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, domainarchive.ErrActiveJobExists):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		respondError(c, err)
	}
}

// parseOptionalDirection 解析 direction=in|out 过滤条件。
func parseOptionalDirection(c *gin.Context) (*bool, bool) {
	switch c.Query("direction") {
	case "":
		return nil, true
	case "in":
		v := false
		return &v, true
	case "out":
		v := true
		return &v, true
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "direction 必须为 in 或 out"})
		return nil, false
	}
}
