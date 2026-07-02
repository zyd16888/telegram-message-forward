package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"telegram-message-forward/internal/api/dto"
	appsource "telegram-message-forward/internal/app/source"
	pluginsource "telegram-message-forward/internal/plugin/source"
)

// SourceHandler 处理监听源相关请求。
type SourceHandler struct {
	svc *appsource.Service
}

// NewSourceHandler 创建监听源 handler。
func NewSourceHandler(svc *appsource.Service) *SourceHandler {
	return &SourceHandler{svc: svc}
}

// List GET /sources
func (h *SourceHandler) List(c *gin.Context) {
	srcs, err := h.svc.List(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]dto.SourceDTO, 0, len(srcs))
	for _, s := range srcs {
		out = append(out, dto.NewSourceDTO(s))
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// Get GET /sources/:id
func (h *SourceHandler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	s, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewSourceDTO(s)})
}

// Create POST /sources
func (h *SourceHandler) Create(c *gin.Context) {
	var req dto.SourceCreateRequest
	if !bindJSON(c, &req) {
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	s, err := h.svc.Create(c.Request.Context(), appsource.CreateInput{
		AccountID: req.AccountID,
		PeerType:  req.PeerType,
		PeerID:    req.PeerID,
		Name:      req.Name,
		Username:  req.Username,
		Enabled:   enabled,
		Config:    req.Config,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": dto.NewSourceDTO(s)})
}

// Update PUT /sources/:id
func (h *SourceHandler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req dto.SourceUpdateRequest
	if !bindJSON(c, &req) {
		return
	}
	s, err := h.svc.Update(c.Request.Context(), id, appsource.UpdateInput{
		Name:    req.Name,
		Enabled: req.Enabled,
		Config:  req.Config,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewSourceDTO(s)})
}

// Delete DELETE /sources/:id
func (h *SourceHandler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// Sync POST /sources/sync?account_id=N — 拉取账号可见 peer 列表。
func (h *SourceHandler) Sync(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Query("account_id"), 10, 64)
	if err != nil || accountID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少或非法 account_id"})
		return
	}
	peers, err := h.svc.Sync(c.Request.Context(), accountID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	out := make([]dto.SyncedPeerDTO, 0, len(peers))
	for _, p := range peers {
		out = append(out, newSyncedPeerDTO(p))
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// SyncStream GET /sources/sync/stream?account_id=N — 流式拉取账号可见 peer 列表。
func (h *SourceHandler) SyncStream(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Query("account_id"), 10, 64)
	if err != nil || accountID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少或非法 account_id"})
		return
	}

	w := c.Writer
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	w.Flush()

	writeEvent := func(event string, payload any) error {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b); err != nil {
			return err
		}
		w.Flush()
		return nil
	}

	count := 0
	err = h.svc.SyncStream(c.Request.Context(), accountID, func(p pluginsource.SyncedPeer) error {
		count++
		return writeEvent("peer", newSyncedPeerDTO(p))
	})
	if err != nil {
		_ = writeEvent("error", gin.H{"error": err.Error()})
		return
	}
	_ = writeEvent("done", gin.H{"count": count})
}

func newSyncedPeerDTO(p pluginsource.SyncedPeer) dto.SyncedPeerDTO {
	return dto.SyncedPeerDTO{
		PeerType:     string(p.PeerType),
		PeerKind:     p.PeerKind,
		PeerID:       p.PeerID,
		Name:         p.Name,
		Username:     p.Username,
		DisplayType:  p.DisplayType,
		IsBot:        p.IsBot,
		IsChannel:    p.IsChannel,
		IsSupergroup: p.IsSupergroup,
		IsForum:      p.IsForum,
		Flags:        p.Flags,
	}
}

// Start POST /sources/:id/start
func (h *SourceHandler) Start(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.Start(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "started"})
}

// Stop POST /sources/:id/stop
func (h *SourceHandler) Stop(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.Stop(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "stopped"})
}
