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

type AdminHandler struct {
	adminService         *service.AdminService
	runtimeConfigService *service.RuntimeConfigService
}

func NewAdminHandler(adminService *service.AdminService, runtimeConfigService *service.RuntimeConfigService) *AdminHandler {
	return &AdminHandler{
		adminService:         adminService,
		runtimeConfigService: runtimeConfigService,
	}
}

func (h *AdminHandler) Me(c *gin.Context) {
	linuxDoUserID, ok := middleware.LinuxDoUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if h.adminService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "admin service unavailable"})
		return
	}

	profile, err := h.adminService.PermissionProfileByLinuxDoUserID(c.Request.Context(), linuxDoUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query admin status failed"})
		return
	}
	c.JSON(http.StatusOK, profile)
}

func (h *AdminHandler) ListUsers(c *gin.Context) {
	_, _, ok := h.requireAdminPermission(c, service.AdminPermissionManageAdmins)
	if !ok {
		return
	}

	limit := 200
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
		limit = parsed
	}

	result, err := h.adminService.ListAdmins(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list admin users failed"})
		return
	}
	c.JSON(http.StatusOK, result)
}

type adminUpsertUserRequest struct {
	LinuxDoUserID string `json:"linuxDoUserId"`
	Role          string `json:"role"`
	Note          string `json:"note"`
}

func (h *AdminHandler) UpsertUser(c *gin.Context) {
	operatorUserID, _, ok := h.requireAdminPermission(c, service.AdminPermissionManageAdmins)
	if !ok {
		return
	}

	var req adminUpsertUserRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.LinuxDoUserID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "linuxDoUserId is required"})
		return
	}

	record, err := h.adminService.UpsertAdmin(c.Request.Context(), service.AdminUserUpsertInput{
		LinuxDoUserID:   req.LinuxDoUserID,
		Role:            req.Role,
		Note:            req.Note,
		Source:          "manual",
		CreatedByUserID: &operatorUserID,
	})
	if err != nil {
		h.handleAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "管理员更新成功",
		"user":    record,
	})
}

func (h *AdminHandler) DeleteUser(c *gin.Context) {
	_, linuxDoUserID, ok := h.requireAdminPermission(c, service.AdminPermissionManageAdmins)
	if !ok {
		return
	}

	targetLinuxDoUserID := strings.TrimSpace(c.Query("linuxDoUserId"))
	if targetLinuxDoUserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "linuxDoUserId is required"})
		return
	}
	if strings.EqualFold(targetLinuxDoUserID, linuxDoUserID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot remove current operator"})
		return
	}

	removed, err := h.adminService.RemoveAdmin(c.Request.Context(), targetLinuxDoUserID)
	if err != nil {
		h.handleAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "管理员移除成功",
		"removed": removed,
	})
}

func (h *AdminHandler) RuntimeConfigs(c *gin.Context) {
	_, _, ok := h.requireAdminPermission(c, service.AdminPermissionManageRuntimeConfigs)
	if !ok {
		return
	}
	if h.runtimeConfigService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "runtime config service unavailable"})
		return
	}

	category := strings.TrimSpace(c.Query("category"))
	keyword := strings.TrimSpace(c.Query("q"))
	limit := 300
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
		limit = parsed
	}

	result, err := h.runtimeConfigService.List(c.Request.Context(), category, keyword, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list runtime configs failed"})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *AdminHandler) RuntimeConfigAudits(c *gin.Context) {
	_, _, ok := h.requireAdminPermission(c, service.AdminPermissionManageRuntimeConfigs)
	if !ok {
		return
	}
	if h.runtimeConfigService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "runtime config service unavailable"})
		return
	}

	key := strings.TrimSpace(c.Query("key"))
	category := strings.TrimSpace(c.Query("category"))
	limit := 200
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
		limit = parsed
	}

	result, err := h.runtimeConfigService.ListAudits(c.Request.Context(), key, category, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list runtime config audits failed"})
		return
	}
	c.JSON(http.StatusOK, result)
}

type adminUpsertRuntimeConfigRequest struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	ValueType   string `json:"valueType"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

func (h *AdminHandler) RuntimeConfigUpsert(c *gin.Context) {
	operatorUserID, _, ok := h.requireAdminPermission(c, service.AdminPermissionManageRuntimeConfigs)
	if !ok {
		return
	}
	if h.runtimeConfigService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "runtime config service unavailable"})
		return
	}

	var req adminUpsertRuntimeConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}
	record, err := h.runtimeConfigService.Upsert(c.Request.Context(), service.RuntimeConfigUpsertInput{
		Key:             req.Key,
		Value:           req.Value,
		ValueType:       req.ValueType,
		Category:        req.Category,
		Description:     req.Description,
		UpdatedByUserID: &operatorUserID,
	})
	if err != nil {
		h.handleRuntimeConfigError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "运行时配置更新成功",
		"config":  record,
	})
}

func (h *AdminHandler) requireAdminPermission(
	c *gin.Context,
	permission service.AdminPermission,
) (userID uuid.UUID, linuxDoUserID string, ok bool) {
	userID, ok = middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return uuid.Nil, "", false
	}
	linuxDoUserID, ok = middleware.LinuxDoUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return uuid.Nil, "", false
	}
	if h.adminService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "admin service unavailable"})
		return uuid.Nil, "", false
	}

	profile, err := h.adminService.PermissionProfileByLinuxDoUserID(c.Request.Context(), linuxDoUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query admin status failed"})
		return uuid.Nil, "", false
	}
	if !profile.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "game admin required"})
		return uuid.Nil, "", false
	}
	if !profile.Has(permission) {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient admin permission"})
		return uuid.Nil, "", false
	}

	return userID, linuxDoUserID, true
}

func (h *AdminHandler) handleAdminError(c *gin.Context, err error) {
	var invalidErr *service.InvalidAdminLinuxDoUserIDError
	if errors.As(err, &invalidErr) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid linuxDoUserId"})
		return
	}

	var invalidRoleErr *service.InvalidAdminRoleError
	if errors.As(err, &invalidRoleErr) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid admin role",
			"role":  invalidRoleErr.Role,
		})
		return
	}

	var notFoundErr *service.AdminUserNotFoundError
	if errors.As(err, &notFoundErr) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":         "admin user not found",
			"linuxDoUserId": notFoundErr.LinuxDoUserID,
		})
		return
	}

	var lastErr *service.AdminLastUserRemoveError
	if errors.As(err, &lastErr) {
		c.JSON(http.StatusConflict, gin.H{
			"error":         "cannot remove last admin",
			"linuxDoUserId": lastErr.LinuxDoUserID,
		})
		return
	}

	var lastSuperRemoveErr *service.AdminLastSuperUserRemoveError
	if errors.As(err, &lastSuperRemoveErr) {
		c.JSON(http.StatusConflict, gin.H{
			"error":         "cannot remove last super admin",
			"linuxDoUserId": lastSuperRemoveErr.LinuxDoUserID,
		})
		return
	}

	var lastSuperDemoteErr *service.AdminLastSuperUserDemoteError
	if errors.As(err, &lastSuperDemoteErr) {
		c.JSON(http.StatusConflict, gin.H{
			"error":         "cannot demote last super admin",
			"linuxDoUserId": lastSuperDemoteErr.LinuxDoUserID,
		})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "admin operation failed"})
}

func (h *AdminHandler) handleRuntimeConfigError(c *gin.Context, err error) {
	var invalidKeyErr *service.InvalidRuntimeConfigKeyError
	if errors.As(err, &invalidKeyErr) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid runtime config key"})
		return
	}

	var invalidTypeErr *service.InvalidRuntimeConfigTypeError
	if errors.As(err, &invalidTypeErr) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":     "invalid runtime config type",
			"valueType": invalidTypeErr.ValueType,
		})
		return
	}

	var invalidValueErr *service.InvalidRuntimeConfigValueError
	if errors.As(err, &invalidValueErr) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "invalid runtime config value",
			"reason": invalidValueErr.Reason,
		})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "runtime config operation failed"})
}
