package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/kowming/vue-idle-xiuxian/backend/internal/http/middleware"
	"github.com/kowming/vue-idle-xiuxian/backend/internal/service"
)

type AuctionHandler struct {
	auctionService *service.AuctionService
}

func NewAuctionHandler(auctionService *service.AuctionService) *AuctionHandler {
	return &AuctionHandler{auctionService: auctionService}
}

func (h *AuctionHandler) List(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	limit := 20
	if rawLimit := c.Query("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
		limit = parsed
	}
	offset := 0
	if rawOffset := c.Query("offset"); rawOffset != "" {
		parsed, err := strconv.Atoi(rawOffset)
		if err != nil || parsed < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid offset"})
			return
		}
		offset = parsed
	}

	result, err := h.auctionService.List(c.Request.Context(), userID, limit, offset)
	if err != nil {
		h.handleAuctionError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *AuctionHandler) MyOrders(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	limit := 20
	if rawLimit := c.Query("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
		limit = parsed
	}

	result, err := h.auctionService.MyOrders(c.Request.Context(), userID, limit)
	if err != nil {
		h.handleAuctionError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

type auctionCreateRequest struct {
	ItemID        string `json:"itemId"`
	Price         int64  `json:"price"`
	DurationHours int    `json:"durationHours"`
}

func (h *AuctionHandler) Create(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req auctionCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.ItemID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "itemId is required"})
		return
	}

	result, err := h.auctionService.Create(c.Request.Context(), userID, req.ItemID, req.Price, req.DurationHours)
	if err != nil {
		h.handleAuctionError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

type auctionOrderActionRequest struct {
	OrderID int64 `json:"orderId"`
}

func (h *AuctionHandler) Cancel(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req auctionOrderActionRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.OrderID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "orderId is required"})
		return
	}

	result, err := h.auctionService.Cancel(c.Request.Context(), userID, req.OrderID)
	if err != nil {
		h.handleAuctionError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *AuctionHandler) Buy(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req auctionOrderActionRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.OrderID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "orderId is required"})
		return
	}

	result, err := h.auctionService.Buy(c.Request.Context(), userID, req.OrderID)
	if err != nil {
		h.handleAuctionError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

type auctionBidRequest struct {
	OrderID int64 `json:"orderId"`
	Amount  int64 `json:"amount"`
}

func (h *AuctionHandler) Bid(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req auctionBidRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.OrderID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "orderId is required"})
		return
	}

	result, err := h.auctionService.Bid(c.Request.Context(), userID, req.OrderID, req.Amount)
	if err != nil {
		h.handleAuctionError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *AuctionHandler) AcceptBid(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req auctionOrderActionRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.OrderID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "orderId is required"})
		return
	}

	result, err := h.auctionService.AcceptHighestBid(c.Request.Context(), userID, req.OrderID)
	if err != nil {
		h.handleAuctionError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *AuctionHandler) handleAuctionError(c *gin.Context, err error) {
	var invalidPriceErr *service.InvalidAuctionPriceError
	if errors.As(err, &invalidPriceErr) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid auction price",
			"price": invalidPriceErr.Price,
		})
		return
	}

	var invalidDurationErr *service.InvalidAuctionDurationError
	if errors.As(err, &invalidDurationErr) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":    "invalid auction duration",
			"duration": invalidDurationErr.DurationHours,
		})
		return
	}

	var invalidBidAmountErr *service.InvalidAuctionBidAmountError
	if errors.As(err, &invalidBidAmountErr) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "invalid auction bid amount",
			"amount": invalidBidAmountErr.Amount,
		})
		return
	}

	var bidTooLowErr *service.AuctionBidTooLowError
	if errors.As(err, &bidTooLowErr) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":        "auction bid too low",
			"amount":       bidTooLowErr.Amount,
			"requiredMore": bidTooLowErr.RequiredMore,
		})
		return
	}

	var noActiveBidErr *service.AuctionOrderNoActiveBidError
	if errors.As(err, &noActiveBidErr) {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "auction order has no active bid",
			"orderId": noActiveBidErr.OrderID,
		})
		return
	}

	var bidderInsufficientErr *service.AuctionBidderInsufficientSpiritStonesError
	if errors.As(err, &bidderInsufficientErr) {
		c.JSON(http.StatusConflict, gin.H{
			"error":    "auction highest bidder insufficient spirit stones",
			"orderId":  bidderInsufficientErr.OrderID,
			"required": bidderInsufficientErr.Required,
		})
		return
	}

	var itemNotFoundErr *service.AuctionItemNotFoundError
	if errors.As(err, &itemNotFoundErr) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":  "auction item not found",
			"itemId": itemNotFoundErr.ItemID,
		})
		return
	}

	var itemNotTradableErr *service.AuctionItemNotTradableError
	if errors.As(err, &itemNotTradableErr) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":    "auction item not tradable",
			"itemId":   itemNotTradableErr.ItemID,
			"itemType": itemNotTradableErr.ItemType,
		})
		return
	}

	var orderNotFoundErr *service.AuctionOrderNotFoundError
	if errors.As(err, &orderNotFoundErr) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "auction order not found",
			"orderId": orderNotFoundErr.OrderID,
		})
		return
	}

	var orderForbiddenErr *service.AuctionOrderForbiddenError
	if errors.As(err, &orderForbiddenErr) {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "auction order forbidden",
			"orderId": orderForbiddenErr.OrderID,
		})
		return
	}

	var selfPurchaseErr *service.AuctionSelfPurchaseError
	if errors.As(err, &selfPurchaseErr) {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "auction self purchase not allowed",
			"orderId": selfPurchaseErr.OrderID,
		})
		return
	}

	var invalidStatusErr *service.AuctionOrderInvalidStatusError
	if errors.As(err, &invalidStatusErr) {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "auction order status invalid",
			"orderId": invalidStatusErr.OrderID,
			"status":  invalidStatusErr.Status,
		})
		return
	}

	var expiredErr *service.AuctionOrderExpiredError
	if errors.As(err, &expiredErr) {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "auction order expired",
			"orderId": expiredErr.OrderID,
		})
		return
	}

	var insufficientErr *service.AuctionInsufficientSpiritStonesError
	if errors.As(err, &insufficientErr) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":    "insufficient spirit stones",
			"required": insufficientErr.Required,
			"current":  insufficientErr.Current,
		})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "auction operation failed"})
}
