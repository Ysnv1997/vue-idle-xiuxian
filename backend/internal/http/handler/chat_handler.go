package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/kowming/vue-idle-xiuxian/backend/internal/http/middleware"
	"github.com/kowming/vue-idle-xiuxian/backend/internal/service"
)

type ChatHandler struct {
	chatService  *service.ChatService
	tokenService *service.TokenService
	adminService *service.AdminService

	upgrader websocket.Upgrader
	clients  map[*websocket.Conn]uuid.UUID
	clientsM sync.RWMutex
	writeM   sync.Mutex
}

func NewChatHandler(chatService *service.ChatService, tokenService *service.TokenService, adminService *service.AdminService) *ChatHandler {
	return &ChatHandler{
		chatService:  chatService,
		tokenService: tokenService,
		adminService: adminService,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(_ *http.Request) bool {
				return true
			},
		},
		clients: make(map[*websocket.Conn]uuid.UUID),
	}
}

func (h *ChatHandler) History(c *gin.Context) {
	_, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	channel := c.DefaultQuery("channel", "world")
	limit := 50
	if rawLimit := c.Query("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
		limit = parsed
	}

	result, err := h.chatService.History(c.Request.Context(), channel, limit)
	if err != nil {
		h.handleChatError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *ChatHandler) MuteStatus(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	status, err := h.chatService.MuteStatus(c.Request.Context(), userID)
	if err != nil {
		h.handleChatError(c, err)
		return
	}

	c.JSON(http.StatusOK, status)
}

type chatReportRequest struct {
	MessageID int64  `json:"messageId"`
	Reason    string `json:"reason"`
}

func (h *ChatHandler) Report(c *gin.Context) {
	reporterID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req chatReportRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.MessageID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "messageId is required"})
		return
	}

	if err := h.chatService.Report(c.Request.Context(), reporterID, req.MessageID, req.Reason); err != nil {
		h.handleChatError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "举报已提交"})
}

type chatAdminMuteRequest struct {
	TargetLinuxDoUserID string `json:"targetLinuxDoUserId"`
	DurationMinutes     int    `json:"durationMinutes"`
	Reason              string `json:"reason"`
}

func (h *ChatHandler) AdminMute(c *gin.Context) {
	operatorUserID, _, ok := h.requireChatAdmin(c)
	if !ok {
		return
	}

	var req chatAdminMuteRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.TargetLinuxDoUserID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "targetLinuxDoUserId is required"})
		return
	}
	if req.DurationMinutes <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "durationMinutes must be positive"})
		return
	}

	status, err := h.chatService.MuteByLinuxDoUserID(
		c.Request.Context(),
		req.TargetLinuxDoUserID,
		req.DurationMinutes,
		req.Reason,
		operatorUserID,
	)
	if err != nil {
		h.handleChatError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "禁言成功",
		"status":  status,
	})
}

type chatAdminUnmuteRequest struct {
	TargetLinuxDoUserID string `json:"targetLinuxDoUserId"`
}

func (h *ChatHandler) AdminUnmute(c *gin.Context) {
	operatorUserID, _, ok := h.requireChatAdmin(c)
	if !ok {
		return
	}

	var req chatAdminUnmuteRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.TargetLinuxDoUserID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "targetLinuxDoUserId is required"})
		return
	}

	updated, err := h.chatService.UnmuteByLinuxDoUserID(c.Request.Context(), req.TargetLinuxDoUserID, operatorUserID)
	if err != nil {
		h.handleChatError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "解除禁言成功",
		"updated": updated,
	})
}

func (h *ChatHandler) AdminMutes(c *gin.Context) {
	_, _, ok := h.requireChatAdmin(c)
	if !ok {
		return
	}

	targetLinuxDoUserID := strings.TrimSpace(c.Query("targetLinuxDoUserId"))
	limit := 50
	if rawLimit := c.Query("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
		limit = parsed
	}

	result, err := h.chatService.ListActiveMutes(c.Request.Context(), targetLinuxDoUserID, limit)
	if err != nil {
		h.handleChatError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *ChatHandler) AdminReports(c *gin.Context) {
	_, _, ok := h.requireChatAdmin(c)
	if !ok {
		return
	}

	status := strings.TrimSpace(c.DefaultQuery("status", "pending"))
	limit := 100
	if rawLimit := c.Query("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
		limit = parsed
	}

	result, err := h.chatService.ListReports(c.Request.Context(), status, limit)
	if err != nil {
		h.handleChatError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

type chatAdminReviewReportRequest struct {
	ReportID int64  `json:"reportId"`
	Status   string `json:"status"`
	Note     string `json:"note"`
}

func (h *ChatHandler) AdminReviewReport(c *gin.Context) {
	operatorUserID, _, ok := h.requireChatAdmin(c)
	if !ok {
		return
	}

	var req chatAdminReviewReportRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.ReportID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reportId is required"})
		return
	}
	if strings.TrimSpace(req.Status) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status is required"})
		return
	}

	if err := h.chatService.ReviewReport(c.Request.Context(), req.ReportID, req.Status, req.Note, operatorUserID); err != nil {
		h.handleChatError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "举报审核已更新"})
}

func (h *ChatHandler) AdminBlockWords(c *gin.Context) {
	_, _, ok := h.requireChatAdmin(c)
	if !ok {
		return
	}

	includeDisabled := strings.EqualFold(strings.TrimSpace(c.Query("includeDisabled")), "true")
	limit := 200
	if rawLimit := c.Query("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
		limit = parsed
	}

	result, err := h.chatService.ListBlockedWords(c.Request.Context(), includeDisabled, limit)
	if err != nil {
		h.handleChatError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

type chatAdminBlockWordUpsertRequest struct {
	Word    string `json:"word"`
	Enabled *bool  `json:"enabled"`
}

func (h *ChatHandler) AdminUpsertBlockWord(c *gin.Context) {
	operatorUserID, _, ok := h.requireChatAdmin(c)
	if !ok {
		return
	}

	var req chatAdminBlockWordUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Word) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "word is required"})
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	record, err := h.chatService.UpsertBlockedWord(c.Request.Context(), req.Word, enabled, operatorUserID)
	if err != nil {
		h.handleChatError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "违禁词更新成功",
		"word":    record,
	})
}

func (h *ChatHandler) AdminDeleteBlockWord(c *gin.Context) {
	operatorUserID, _, ok := h.requireChatAdmin(c)
	if !ok {
		return
	}

	word := strings.TrimSpace(c.Query("word"))
	if word == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "word is required"})
		return
	}

	removed, err := h.chatService.DeleteBlockedWord(c.Request.Context(), word, operatorUserID)
	if err != nil {
		h.handleChatError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "违禁词删除成功",
		"removed": removed,
		"word":    word,
	})
}

func (h *ChatHandler) Connect(c *gin.Context) {
	accessToken := strings.TrimSpace(c.Query("accessToken"))
	if accessToken == "" {
		header := strings.TrimSpace(c.GetHeader("Authorization"))
		parts := strings.SplitN(header, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			accessToken = strings.TrimSpace(parts[1])
		}
	}
	if accessToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing access token"})
		return
	}

	claims, err := h.tokenService.ValidateToken(accessToken, "access")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid access token"})
		return
	}
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id in token"})
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	h.registerClient(conn, userID)
	defer func() {
		h.unregisterClient(conn)
		_ = conn.Close()
	}()

	h.writeEnvelope(conn, "chat.connected", gin.H{"channel": "world"})

	for {
		var incoming wsIncomingEnvelope
		if err := conn.ReadJSON(&incoming); err != nil {
			break
		}

		switch incoming.Event {
		case "chat.send":
			if err := h.handleWSSendEvent(conn, userID, incoming.Data); err != nil {
				h.writeEnvelope(conn, "chat.error", gin.H{"error": err.Error()})
			}
		default:
			h.writeEnvelope(conn, "chat.error", gin.H{"error": "unsupported event"})
		}
	}
}

type wsIncomingEnvelope struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

type wsSendPayload struct {
	Channel string `json:"channel"`
	Content string `json:"content"`
}

func (h *ChatHandler) handleWSSendEvent(conn *websocket.Conn, userID uuid.UUID, payloadRaw json.RawMessage) error {
	var payload wsSendPayload
	if err := json.Unmarshal(payloadRaw, &payload); err != nil {
		return errors.New("invalid chat.send payload")
	}

	message, err := h.chatService.Send(context.Background(), userID, payload.Channel, payload.Content)
	if err != nil {
		return err
	}

	h.writeEnvelope(conn, "chat.sent", message)
	h.broadcast("chat.receive", message)
	return nil
}

func (h *ChatHandler) registerClient(conn *websocket.Conn, userID uuid.UUID) {
	h.clientsM.Lock()
	defer h.clientsM.Unlock()
	h.clients[conn] = userID
}

func (h *ChatHandler) unregisterClient(conn *websocket.Conn) {
	h.clientsM.Lock()
	defer h.clientsM.Unlock()
	delete(h.clients, conn)
}

func (h *ChatHandler) broadcast(event string, data any) {
	h.clientsM.RLock()
	conns := make([]*websocket.Conn, 0, len(h.clients))
	for conn := range h.clients {
		conns = append(conns, conn)
	}
	h.clientsM.RUnlock()

	for _, conn := range conns {
		if err := h.writeEnvelope(conn, event, data); err != nil {
			h.unregisterClient(conn)
			_ = conn.Close()
		}
	}
}

func (h *ChatHandler) writeEnvelope(conn *websocket.Conn, event string, data any) error {
	h.writeM.Lock()
	defer h.writeM.Unlock()

	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return conn.WriteJSON(gin.H{
		"event": event,
		"data":  data,
	})
}

func (h *ChatHandler) requireChatAdmin(c *gin.Context) (uuid.UUID, string, bool) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return uuid.Nil, "", false
	}
	linuxDoUserID, ok := middleware.LinuxDoUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return uuid.Nil, "", false
	}
	if h.adminService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "chat admin unavailable"})
		return uuid.Nil, "", false
	}

	allowed, err := h.adminService.HasPermissionByLinuxDoUserID(
		c.Request.Context(),
		linuxDoUserID,
		service.AdminPermissionModerateChat,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query admin status failed"})
		return uuid.Nil, "", false
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "chat admin required"})
		return uuid.Nil, "", false
	}
	return userID, linuxDoUserID, true
}

func (h *ChatHandler) handleChatError(c *gin.Context, err error) {
	var invalidChannelErr *service.InvalidChatChannelError
	if errors.As(err, &invalidChannelErr) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid chat channel",
			"channel": invalidChannelErr.Channel,
		})
		return
	}

	var invalidContentErr *service.InvalidChatContentError
	if errors.As(err, &invalidContentErr) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "invalid chat content",
			"reason": invalidContentErr.Reason,
		})
		return
	}

	var rateLimitedErr *service.ChatRateLimitedError
	if errors.As(err, &rateLimitedErr) {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":           "chat rate limited",
			"retryAfterMilli": rateLimitedErr.RetryAfter.Milliseconds(),
		})
		return
	}

	var mutedErr *service.ChatMutedError
	if errors.As(err, &mutedErr) {
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "chat muted",
			"mutedUntil": mutedErr.MutedUntil,
		})
		return
	}

	var notFoundErr *service.ChatMessageNotFoundError
	if errors.As(err, &notFoundErr) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":     "chat message not found",
			"messageId": notFoundErr.MessageID,
		})
		return
	}

	var targetNotFoundErr *service.ChatTargetUserNotFoundError
	if errors.As(err, &targetNotFoundErr) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":               "chat target user not found",
			"targetLinuxDoUserId": targetNotFoundErr.LinuxDoUserID,
		})
		return
	}

	var invalidDurationErr *service.InvalidChatMuteDurationError
	if errors.As(err, &invalidDurationErr) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":           "invalid mute duration",
			"durationMinutes": invalidDurationErr.DurationMinutes,
		})
		return
	}

	var invalidBlockedWordErr *service.InvalidChatBlockedWordError
	if errors.As(err, &invalidBlockedWordErr) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "invalid blocked word",
			"reason": invalidBlockedWordErr.Reason,
		})
		return
	}

	var reportNotFoundErr *service.ChatReportNotFoundError
	if errors.As(err, &reportNotFoundErr) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":    "chat report not found",
			"reportId": reportNotFoundErr.ReportID,
		})
		return
	}

	var invalidReviewStatusErr *service.InvalidChatReportReviewStatusError
	if errors.As(err, &invalidReviewStatusErr) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "invalid chat report review status",
			"status": invalidReviewStatusErr.Status,
		})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "chat operation failed"})
}
