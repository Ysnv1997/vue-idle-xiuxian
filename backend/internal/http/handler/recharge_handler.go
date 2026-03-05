package handler

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/kowming/vue-idle-xiuxian/backend/internal/http/middleware"
	"github.com/kowming/vue-idle-xiuxian/backend/internal/service"
)

type RechargeHandler struct {
	rechargeService   *service.RechargeService
	enableMockPayment bool
}

func NewRechargeHandler(rechargeService *service.RechargeService, enableMockPayment bool) *RechargeHandler {
	return &RechargeHandler{
		rechargeService:   rechargeService,
		enableMockPayment: enableMockPayment,
	}
}

func (h *RechargeHandler) Products(c *gin.Context) {
	_, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	result, err := h.rechargeService.ListProducts(c.Request.Context(), false)
	if err != nil {
		h.handleRechargeError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *RechargeHandler) Orders(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	limit := 20
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
		limit = parsed
	}

	result, err := h.rechargeService.ListOrders(c.Request.Context(), userID, limit)
	if err != nil {
		h.handleRechargeError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

type rechargeCreateOrderRequest struct {
	ProductCode    string `json:"productCode"`
	IdempotencyKey string `json:"idempotencyKey"`
}

func (h *RechargeHandler) CreateOrder(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req rechargeCreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.ProductCode) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "productCode is required"})
		return
	}
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		req.IdempotencyKey = strings.TrimSpace(c.GetHeader("X-Idempotency-Key"))
	}

	result, err := h.rechargeService.CreateOrder(c.Request.Context(), userID, service.RechargeCreateInput{
		ProductCode:    req.ProductCode,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		h.handleRechargeError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *RechargeHandler) CreditLinuxDoCallback(c *gin.Context) {
	params := make(map[string]string, len(c.Request.URL.Query()))
	for key, values := range c.Request.URL.Query() {
		if len(values) == 0 {
			continue
		}
		params[key] = values[0]
	}

	// 浏览器误访问回调地址（或 return_url 指向了回调）时，直接回到前端充值页。
	if strings.TrimSpace(params["sign"]) == "" &&
		strings.TrimSpace(params["out_trade_no"]) == "" &&
		strings.TrimSpace(params["trade_status"]) == "" {
		c.Redirect(http.StatusFound, "/#/recharge")
		return
	}

	_, err := h.rechargeService.HandleCreditLinuxDoCallback(c.Request.Context(), params)
	if err != nil {
		log.Printf("recharge callback failed: err=%v params=%v", err, params)
		c.String(http.StatusBadRequest, "fail")
		return
	}
	c.String(http.StatusOK, "success")
}

type rechargeMockPaidRequest struct {
	OrderID int64 `json:"orderId"`
}

func (h *RechargeHandler) MockPaid(c *gin.Context) {
	if !h.enableMockPayment {
		c.JSON(http.StatusNotFound, gin.H{"error": "mock payment is disabled"})
		return
	}

	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req rechargeMockPaidRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.OrderID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "orderId is required"})
		return
	}

	result, err := h.rechargeService.MockMarkPaid(c.Request.Context(), userID, req.OrderID)
	if err != nil {
		h.handleRechargeError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

type rechargeSyncOrderRequest struct {
	OrderID int64 `json:"orderId"`
}

func (h *RechargeHandler) SyncOrder(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req rechargeSyncOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.OrderID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "orderId is required"})
		return
	}

	result, err := h.rechargeService.SyncOrder(c.Request.Context(), userID, req.OrderID)
	if err != nil {
		h.handleRechargeError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *RechargeHandler) handleRechargeError(c *gin.Context, err error) {
	var productNotFoundErr *service.RechargeProductNotFoundError
	if errors.As(err, &productNotFoundErr) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":       "recharge product not found",
			"productCode": productNotFoundErr.ProductCode,
		})
		return
	}

	var productDisabledErr *service.RechargeProductDisabledError
	if errors.As(err, &productDisabledErr) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":       "recharge product disabled",
			"productCode": productDisabledErr.ProductCode,
		})
		return
	}

	var orderNotFoundErr *service.RechargeOrderNotFoundError
	if errors.As(err, &orderNotFoundErr) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":           "recharge order not found",
			"orderId":         orderNotFoundErr.OrderID,
			"externalOrderId": orderNotFoundErr.ExternalOrderID,
		})
		return
	}

	var orderForbiddenErr *service.RechargeOrderForbiddenError
	if errors.As(err, &orderForbiddenErr) {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "recharge order forbidden",
			"orderId": orderForbiddenErr.OrderID,
		})
		return
	}

	var conflictErr *service.RechargeIdempotencyConflictError
	if errors.As(err, &conflictErr) {
		c.JSON(http.StatusConflict, gin.H{
			"error":          "recharge idempotency conflict",
			"idempotencyKey": conflictErr.IdempotencyKey,
		})
		return
	}

	var invalidSignatureErr *service.RechargeInvalidCallbackSignatureError
	if errors.As(err, &invalidSignatureErr) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid callback signature",
		})
		return
	}

	var invalidPayloadErr *service.RechargeInvalidCallbackPayloadError
	if errors.As(err, &invalidPayloadErr) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "invalid callback payload",
			"reason": invalidPayloadErr.Reason,
		})
		return
	}

	var providerConfigErr *service.RechargeProviderConfigError
	if errors.As(err, &providerConfigErr) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "recharge provider config invalid",
			"reason": providerConfigErr.Reason,
		})
		return
	}

	var providerRequestErr *service.RechargeProviderRequestError
	if errors.As(err, &providerRequestErr) {
		c.JSON(http.StatusBadGateway, gin.H{
			"error":  "recharge provider request failed",
			"reason": providerRequestErr.Reason,
		})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "recharge operation failed"})
}
