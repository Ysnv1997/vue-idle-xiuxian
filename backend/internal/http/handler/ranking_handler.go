package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/kowming/vue-idle-xiuxian/backend/internal/http/middleware"
	"github.com/kowming/vue-idle-xiuxian/backend/internal/service"
)

type RankingHandler struct {
	rankingService *service.RankingService
}

func NewRankingHandler(rankingService *service.RankingService) *RankingHandler {
	return &RankingHandler{rankingService: rankingService}
}

func (h *RankingHandler) Rankings(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	rankingType := c.DefaultQuery("type", "realm")
	scope := c.DefaultQuery("scope", "global")
	limit, ok := parsePositiveLimit(c, 50)
	if !ok {
		return
	}

	result, err := h.rankingService.List(c.Request.Context(), userID, rankingType, scope, limit)
	if err != nil {
		h.handleRankingError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *RankingHandler) RankingFriends(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	rankingType := c.DefaultQuery("type", "realm")
	limit, ok := parsePositiveLimit(c, 50)
	if !ok {
		return
	}

	result, err := h.rankingService.List(c.Request.Context(), userID, rankingType, "friends", limit)
	if err != nil {
		h.handleRankingError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *RankingHandler) RankingSelf(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	rankingType := c.DefaultQuery("type", "realm")
	scope := c.DefaultQuery("scope", "global")
	entry, err := h.rankingService.Self(c.Request.Context(), userID, rankingType, scope)
	if err != nil {
		h.handleRankingError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"type":  rankingType,
		"scope": scope,
		"self":  entry,
	})
}

func (h *RankingHandler) Follows(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	limit, ok := parsePositiveLimit(c, 100)
	if !ok {
		return
	}

	result, err := h.rankingService.ListFollows(c.Request.Context(), userID, limit)
	if err != nil {
		h.handleRankingError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

type rankingFollowRequest struct {
	TargetUserID string `json:"targetUserId"`
}

func (h *RankingHandler) Follow(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req rankingFollowRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.TargetUserID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "targetUserId is required"})
		return
	}

	targetUserID, err := uuid.Parse(strings.TrimSpace(req.TargetUserID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid targetUserId"})
		return
	}

	created, err := h.rankingService.Follow(c.Request.Context(), userID, targetUserID)
	if err != nil {
		h.handleRankingError(c, err)
		return
	}

	message := "已关注"
	if created {
		message = "关注成功"
	}
	c.JSON(http.StatusOK, gin.H{
		"message":  message,
		"followed": created,
	})
}

func (h *RankingHandler) Unfollow(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	targetUserIDRaw := strings.TrimSpace(c.Query("targetUserId"))
	if targetUserIDRaw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "targetUserId is required"})
		return
	}
	targetUserID, err := uuid.Parse(targetUserIDRaw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid targetUserId"})
		return
	}

	removed, err := h.rankingService.Unfollow(c.Request.Context(), userID, targetUserID)
	if err != nil {
		h.handleRankingError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "取消关注成功",
		"removed": removed,
		"userId":  targetUserID,
	})
}

func (h *RankingHandler) handleRankingError(c *gin.Context, err error) {
	var invalidTypeErr *service.InvalidRankingTypeError
	if errors.As(err, &invalidTypeErr) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid ranking type",
			"type":  invalidTypeErr.Type,
		})
		return
	}

	var invalidScopeErr *service.InvalidRankingScopeError
	if errors.As(err, &invalidScopeErr) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid ranking scope",
			"scope": invalidScopeErr.Scope,
		})
		return
	}

	var followSelfErr *service.RankingFollowSelfNotAllowedError
	if errors.As(err, &followSelfErr) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "ranking follow self not allowed",
		})
		return
	}

	var targetNotFoundErr *service.RankingFollowTargetNotFoundError
	if errors.As(err, &targetNotFoundErr) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":        "ranking follow target not found",
			"targetUserId": targetNotFoundErr.TargetUserID,
		})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "rankings query failed"})
}

func parsePositiveLimit(c *gin.Context, fallback int) (int, bool) {
	limit := fallback
	if rawLimit := c.Query("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return 0, false
		}
		limit = parsed
	}
	return limit, true
}
