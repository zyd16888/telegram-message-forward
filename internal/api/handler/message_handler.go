package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"telegram-message-forward/internal/api/dto"
	appmessage "telegram-message-forward/internal/app/message"
	domainmessage "telegram-message-forward/internal/domain/message"
)

type MessageHandler struct {
	svc *appmessage.Service
}

func NewMessageHandler(svc *appmessage.Service) *MessageHandler {
	return &MessageHandler{svc: svc}
}

func (h *MessageHandler) List(c *gin.Context) {
	from, ok := parseOptionalTime(c, "from")
	if !ok {
		return
	}
	to, ok := parseOptionalTime(c, "to")
	if !ok {
		return
	}
	var hasMedia *bool
	if raw := c.Query("has_media"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "has_media 必须为 true 或 false"})
			return
		}
		hasMedia = &value
	}
	query := domainmessage.Query{
		SourceID: parseInt64Default(c.Query("source_id"), 0), SourceType: c.Query("source_type"),
		MessageType: c.Query("message_type"), Keyword: c.Query("q"), HasMedia: hasMedia,
		DeliveryStatus: c.Query("delivery_status"), From: from, To: to,
		BeforeID: parseInt64Default(c.Query("before_id"), 0), Limit: parseIntDefault(c.Query("limit"), 30),
	}
	items, hasMore, err := h.svc.List(c.Request.Context(), query)
	if err != nil {
		h.respond(c, err)
		return
	}
	out := make([]dto.MessageDTO, 0, len(items))
	for i := range items {
		out = append(out, dto.NewMessageDTO(&items[i]))
	}
	var nextCursor int64
	if hasMore && len(items) > 0 {
		nextCursor = items[len(items)-1].Message.ID
	}
	c.JSON(http.StatusOK, gin.H{"data": out, "has_more": hasMore, "next_cursor": nextCursor})
}

func (h *MessageHandler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	detail, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		h.respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewMessageDetailDTO(detail)})
}

func (h *MessageHandler) respond(c *gin.Context, err error) {
	if errors.Is(err, appmessage.ErrInvalidQuery) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	respondError(c, err)
}

func parseOptionalTime(c *gin.Context, key string) (*time.Time, bool) {
	raw := c.Query(key)
	if raw == "" {
		return nil, true
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": key + " 必须为 RFC3339 时间"})
		return nil, false
	}
	return &value, true
}

// parseOptionalTimeValue 解析请求体里的可选 RFC3339 时间字段。
func parseOptionalTimeValue(c *gin.Context, field, raw string) (*time.Time, bool) {
	if raw == "" {
		return nil, true
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": field + " 必须为 RFC3339 时间"})
		return nil, false
	}
	return &value, true
}
