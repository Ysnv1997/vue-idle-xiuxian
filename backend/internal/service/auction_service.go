package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kowming/vue-idle-xiuxian/backend/internal/repository"
)

const (
	auctionDefaultDurationHours = 24
	auctionFeeRate              = 0.05
)

var auctionAllowedDurations = map[int]struct{}{
	6:  {},
	12: {},
	24: {},
}

type AuctionService struct {
	pool     *pgxpool.Pool
	userRepo *repository.UserRepository
}

func NewAuctionService(pool *pgxpool.Pool, userRepo *repository.UserRepository) *AuctionService {
	return &AuctionService{pool: pool, userRepo: userRepo}
}

type AuctionOrder struct {
	ID                  int64          `json:"id"`
	SellerUserID        string         `json:"sellerUserId"`
	SellerName          string         `json:"sellerName"`
	BuyerUserID         string         `json:"buyerUserId,omitempty"`
	HighestBid          int64          `json:"highestBid,omitempty"`
	HighestBidderUserID string         `json:"highestBidderUserId,omitempty"`
	ItemID              string         `json:"itemId"`
	Item                map[string]any `json:"item"`
	Price               int64          `json:"price"`
	FeeRate             float64        `json:"feeRate"`
	FeeAmount           int64          `json:"feeAmount"`
	SellerIncome        int64          `json:"sellerIncome"`
	Status              string         `json:"status"`
	ExpiresAt           *time.Time     `json:"expiresAt,omitempty"`
	CreatedAt           time.Time      `json:"createdAt"`
	UpdatedAt           time.Time      `json:"updatedAt"`
	ClosedAt            *time.Time     `json:"closedAt,omitempty"`
	IsMine              bool           `json:"isMine"`
}

type AuctionListResult struct {
	Orders []AuctionOrder `json:"orders"`
}

type AuctionActionResult struct {
	Message  string                     `json:"message"`
	Order    *AuctionOrder              `json:"order,omitempty"`
	Snapshot *repository.PlayerSnapshot `json:"snapshot,omitempty"`
}

type AuctionSweepResult struct {
	ProcessedOrders int `json:"processedOrders"`
}

type InvalidAuctionPriceError struct {
	Price int64
}

func (e *InvalidAuctionPriceError) Error() string {
	return fmt.Sprintf("invalid auction price: %d", e.Price)
}

type InvalidAuctionDurationError struct {
	DurationHours int
}

func (e *InvalidAuctionDurationError) Error() string {
	return fmt.Sprintf("invalid auction duration hours: %d", e.DurationHours)
}

type AuctionItemNotFoundError struct {
	ItemID string
}

func (e *AuctionItemNotFoundError) Error() string {
	return fmt.Sprintf("auction item not found: %s", e.ItemID)
}

type AuctionItemNotTradableError struct {
	ItemID   string
	ItemType string
}

func (e *AuctionItemNotTradableError) Error() string {
	return fmt.Sprintf("auction item not tradable: %s (%s)", e.ItemID, e.ItemType)
}

type AuctionOrderNotFoundError struct {
	OrderID int64
}

func (e *AuctionOrderNotFoundError) Error() string {
	return fmt.Sprintf("auction order not found: %d", e.OrderID)
}

type AuctionOrderForbiddenError struct {
	OrderID int64
}

func (e *AuctionOrderForbiddenError) Error() string {
	return fmt.Sprintf("auction order forbidden: %d", e.OrderID)
}

type AuctionOrderInvalidStatusError struct {
	OrderID int64
	Status  string
}

func (e *AuctionOrderInvalidStatusError) Error() string {
	return fmt.Sprintf("auction order invalid status: %d (%s)", e.OrderID, e.Status)
}

type AuctionOrderExpiredError struct {
	OrderID int64
}

func (e *AuctionOrderExpiredError) Error() string {
	return fmt.Sprintf("auction order expired: %d", e.OrderID)
}

type AuctionSelfPurchaseError struct {
	OrderID int64
}

func (e *AuctionSelfPurchaseError) Error() string {
	return fmt.Sprintf("auction self purchase not allowed: %d", e.OrderID)
}

type AuctionInsufficientSpiritStonesError struct {
	Required int64
	Current  int64
}

func (e *AuctionInsufficientSpiritStonesError) Error() string {
	return fmt.Sprintf("insufficient spirit stones: required %d current %d", e.Required, e.Current)
}

type InvalidAuctionBidAmountError struct {
	Amount int64
}

func (e *InvalidAuctionBidAmountError) Error() string {
	return fmt.Sprintf("invalid auction bid amount: %d", e.Amount)
}

type AuctionBidTooLowError struct {
	Amount       int64
	RequiredMore int64
}

func (e *AuctionBidTooLowError) Error() string {
	return fmt.Sprintf("auction bid too low: amount %d required_more_than %d", e.Amount, e.RequiredMore)
}

type AuctionOrderNoActiveBidError struct {
	OrderID int64
}

func (e *AuctionOrderNoActiveBidError) Error() string {
	return fmt.Sprintf("auction order has no active bid: %d", e.OrderID)
}

type AuctionBidderInsufficientSpiritStonesError struct {
	OrderID  int64
	Required int64
	BidderID uuid.UUID
}

func (e *AuctionBidderInsufficientSpiritStonesError) Error() string {
	return fmt.Sprintf("auction highest bidder balance changed: order %d required %d bidder %s", e.OrderID, e.Required, e.BidderID.String())
}

func (s *AuctionService) SweepExpired(ctx context.Context, limit int) (*AuctionSweepResult, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin auction sweep transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const query = `
		SELECT id, seller_user_id, item_payload
		FROM auction_orders
		WHERE status = 'open'
		  AND expires_at IS NOT NULL
		  AND expires_at <= now()
		ORDER BY expires_at ASC, id ASC
		FOR UPDATE SKIP LOCKED
		LIMIT $1
	`

	rows, err := tx.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("query expired auction orders: %w", err)
	}
	defer rows.Close()

	type expiredOrder struct {
		ID           int64
		SellerUserID uuid.UUID
		Item         map[string]any
	}

	orders := make([]expiredOrder, 0, limit)
	for rows.Next() {
		var (
			order   expiredOrder
			itemRaw []byte
		)
		if err := rows.Scan(&order.ID, &order.SellerUserID, &itemRaw); err != nil {
			return nil, fmt.Errorf("scan expired auction order: %w", err)
		}
		if err := json.Unmarshal(itemRaw, &order.Item); err != nil || order.Item == nil {
			order.Item = map[string]any{}
		}
		orders = append(orders, order)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate expired auction orders: %w", rows.Err())
	}

	processed := 0
	for _, order := range orders {
		items, err := s.loadInventoryItemsForUpdate(ctx, tx, order.SellerUserID)
		if err != nil {
			return nil, err
		}
		items = append(items, order.Item)
		if err := s.updateInventoryItems(ctx, tx, order.SellerUserID, items); err != nil {
			return nil, err
		}
		if err := s.closeOrderWithStatus(ctx, tx, order.ID, "expired", uuid.Nil); err != nil {
			return nil, err
		}
		processed++
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit auction sweep transaction: %w", err)
	}

	return &AuctionSweepResult{ProcessedOrders: processed}, nil
}

func (s *AuctionService) List(ctx context.Context, userID uuid.UUID, limit int, offset int) (*AuctionListResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	const query = `
		SELECT
			ao.id,
			ao.seller_user_id::text,
			COALESCE(sp.player_name, ''),
			COALESCE(ao.buyer_user_id::text, ''),
			COALESCE(hb.amount, 0),
			COALESCE(hb.bidder_user_id, ''),
			ao.item_id,
			ao.item_payload,
			ao.price,
			ao.fee_rate,
			ao.fee_amount,
			ao.seller_income,
			ao.status,
			ao.expires_at,
			ao.created_at,
			ao.updated_at,
			ao.closed_at
		FROM auction_orders ao
		JOIN player_profiles sp ON sp.user_id = ao.seller_user_id
		LEFT JOIN LATERAL (
			SELECT
				ab.amount,
				ab.bidder_user_id::text AS bidder_user_id
			FROM auction_bids ab
			WHERE ab.order_id = ao.id
			  AND ab.status = 'active'
			ORDER BY ab.amount DESC, ab.id DESC
			LIMIT 1
		) hb ON true
		WHERE ao.status = 'open'
		  AND (ao.expires_at IS NULL OR ao.expires_at > now())
		ORDER BY ao.created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := s.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query auction list: %w", err)
	}
	defer rows.Close()

	orders := make([]AuctionOrder, 0, limit)
	for rows.Next() {
		order, scanErr := scanAuctionOrder(rows, userID)
		if scanErr != nil {
			return nil, scanErr
		}
		orders = append(orders, order)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate auction list rows: %w", rows.Err())
	}

	return &AuctionListResult{Orders: orders}, nil
}

func (s *AuctionService) MyOrders(ctx context.Context, userID uuid.UUID, limit int) (*AuctionListResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	const query = `
		SELECT
			ao.id,
			ao.seller_user_id::text,
			COALESCE(sp.player_name, ''),
			COALESCE(ao.buyer_user_id::text, ''),
			COALESCE(hb.amount, 0),
			COALESCE(hb.bidder_user_id, ''),
			ao.item_id,
			ao.item_payload,
			ao.price,
			ao.fee_rate,
			ao.fee_amount,
			ao.seller_income,
			ao.status,
			ao.expires_at,
			ao.created_at,
			ao.updated_at,
			ao.closed_at
		FROM auction_orders ao
		JOIN player_profiles sp ON sp.user_id = ao.seller_user_id
		LEFT JOIN LATERAL (
			SELECT
				ab.amount,
				ab.bidder_user_id::text AS bidder_user_id
			FROM auction_bids ab
			WHERE ab.order_id = ao.id
			  AND ab.status = 'active'
			ORDER BY ab.amount DESC, ab.id DESC
			LIMIT 1
		) hb ON true
		WHERE ao.seller_user_id = $1 OR ao.buyer_user_id = $1
		ORDER BY ao.created_at DESC
		LIMIT $2
	`

	rows, err := s.pool.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("query my auction orders: %w", err)
	}
	defer rows.Close()

	orders := make([]AuctionOrder, 0, limit)
	for rows.Next() {
		order, scanErr := scanAuctionOrder(rows, userID)
		if scanErr != nil {
			return nil, scanErr
		}
		orders = append(orders, order)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate my auction rows: %w", rows.Err())
	}

	return &AuctionListResult{Orders: orders}, nil
}

func (s *AuctionService) Create(ctx context.Context, userID uuid.UUID, itemID string, price int64, durationHours int) (*AuctionActionResult, error) {
	if price <= 0 {
		return nil, &InvalidAuctionPriceError{Price: price}
	}

	duration, err := auctionNormalizeDuration(durationHours)
	if err != nil {
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin auction create transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	items, err := s.loadInventoryItemsForUpdate(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	index := auctionFindItemIndex(items, itemID)
	if index < 0 {
		return nil, &AuctionItemNotFoundError{ItemID: itemID}
	}

	item := items[index]
	itemType := auctionReadString(item["type"])
	if !auctionIsTradableType(itemType) {
		return nil, &AuctionItemNotTradableError{ItemID: itemID, ItemType: itemType}
	}

	nextItems := make([]map[string]any, 0, len(items)-1)
	nextItems = append(nextItems, items[:index]...)
	nextItems = append(nextItems, items[index+1:]...)
	if err := s.updateInventoryItems(ctx, tx, userID, nextItems); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	expiresAt := now.Add(duration)
	feeAmount := price * 5 / 100
	sellerIncome := price - feeAmount

	const insertSQL = `
		INSERT INTO auction_orders (
			seller_user_id,
			item_id,
			item_payload,
			price,
			fee_rate,
			fee_amount,
			seller_income,
			status,
			expires_at,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3::jsonb, $4, $5, $6, $7, 'open', $8, $9, $9)
		RETURNING id
	`

	itemJSON, err := json.Marshal(item)
	if err != nil {
		return nil, fmt.Errorf("marshal auction item payload: %w", err)
	}

	var orderID int64
	if err := tx.QueryRow(ctx, insertSQL, userID, itemID, string(itemJSON), price, auctionFeeRate, feeAmount, sellerIncome, expiresAt, now).Scan(&orderID); err != nil {
		return nil, fmt.Errorf("insert auction order: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit auction create transaction: %w", err)
	}

	order, err := s.getOrderByID(ctx, orderID, userID)
	if err != nil {
		return nil, err
	}
	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &AuctionActionResult{
		Message:  "上架成功",
		Order:    order,
		Snapshot: snapshot,
	}, nil
}

func (s *AuctionService) Cancel(ctx context.Context, userID uuid.UUID, orderID int64) (*AuctionActionResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin auction cancel transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	orderState, err := s.loadOrderForUpdate(ctx, tx, orderID)
	if err != nil {
		return nil, err
	}
	if orderState.SellerUserID != userID {
		return nil, &AuctionOrderForbiddenError{OrderID: orderID}
	}
	if orderState.Status != "open" {
		return nil, &AuctionOrderInvalidStatusError{OrderID: orderID, Status: orderState.Status}
	}

	items, err := s.loadInventoryItemsForUpdate(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	items = append(items, orderState.Item)
	if err := s.updateInventoryItems(ctx, tx, userID, items); err != nil {
		return nil, err
	}

	if err := s.closeOrderWithStatus(ctx, tx, orderID, "cancelled", uuid.Nil); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit auction cancel transaction: %w", err)
	}

	order, err := s.getOrderByID(ctx, orderID, userID)
	if err != nil {
		return nil, err
	}
	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &AuctionActionResult{
		Message:  "取消上架成功",
		Order:    order,
		Snapshot: snapshot,
	}, nil
}

func (s *AuctionService) Buy(ctx context.Context, userID uuid.UUID, orderID int64) (*AuctionActionResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin auction buy transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	orderState, err := s.loadOrderForUpdate(ctx, tx, orderID)
	if err != nil {
		return nil, err
	}
	if orderState.Status != "open" {
		return nil, &AuctionOrderInvalidStatusError{OrderID: orderID, Status: orderState.Status}
	}
	if orderState.ExpiresAt != nil && orderState.ExpiresAt.Before(time.Now().UTC()) {
		return nil, &AuctionOrderExpiredError{OrderID: orderID}
	}
	if orderState.SellerUserID == userID {
		return nil, &AuctionSelfPurchaseError{OrderID: orderID}
	}

	buyerBalance, err := s.loadSpiritStonesForUpdate(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	if buyerBalance < orderState.Price {
		return nil, &AuctionInsufficientSpiritStonesError{Required: orderState.Price, Current: buyerBalance}
	}

	sellerBalance, err := s.loadSpiritStonesForUpdate(ctx, tx, orderState.SellerUserID)
	if err != nil {
		return nil, err
	}

	buyerItems, err := s.loadInventoryItemsForUpdate(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	buyerItems = append(buyerItems, orderState.Item)
	if err := s.updateInventoryItems(ctx, tx, userID, buyerItems); err != nil {
		return nil, err
	}

	nextBuyerBalance := buyerBalance - orderState.Price
	nextSellerBalance := sellerBalance + orderState.SellerIncome
	if err := s.updateSpiritStones(ctx, tx, userID, nextBuyerBalance); err != nil {
		return nil, err
	}
	if err := s.updateSpiritStones(ctx, tx, orderState.SellerUserID, nextSellerBalance); err != nil {
		return nil, err
	}

	if err := s.insertEconomyLogTx(ctx, tx, userID, "auction_buy", -orderState.Price, nextBuyerBalance, fmt.Sprintf("auction_order:%d", orderID)); err != nil {
		return nil, err
	}
	if err := s.insertEconomyLogTx(ctx, tx, orderState.SellerUserID, "auction_sell", orderState.SellerIncome, nextSellerBalance, fmt.Sprintf("auction_order:%d", orderID)); err != nil {
		return nil, err
	}

	if err := s.closeOrderWithStatus(ctx, tx, orderID, "sold", userID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit auction buy transaction: %w", err)
	}

	order, err := s.getOrderByID(ctx, orderID, userID)
	if err != nil {
		return nil, err
	}
	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &AuctionActionResult{
		Message:  "购买成功",
		Order:    order,
		Snapshot: snapshot,
	}, nil
}

func (s *AuctionService) AcceptHighestBid(ctx context.Context, userID uuid.UUID, orderID int64) (*AuctionActionResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin auction accept bid transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	orderState, err := s.loadOrderForUpdate(ctx, tx, orderID)
	if err != nil {
		return nil, err
	}
	if orderState.SellerUserID != userID {
		return nil, &AuctionOrderForbiddenError{OrderID: orderID}
	}
	if orderState.Status != "open" {
		return nil, &AuctionOrderInvalidStatusError{OrderID: orderID, Status: orderState.Status}
	}
	if orderState.ExpiresAt != nil && orderState.ExpiresAt.Before(time.Now().UTC()) {
		return nil, &AuctionOrderExpiredError{OrderID: orderID}
	}

	highestBid, err := s.loadHighestActiveBidForUpdate(ctx, tx, orderID)
	if err != nil {
		return nil, err
	}
	if highestBid == nil {
		return nil, &AuctionOrderNoActiveBidError{OrderID: orderID}
	}

	buyerBalance, err := s.loadSpiritStonesForUpdate(ctx, tx, highestBid.Bidder)
	if err != nil {
		return nil, err
	}
	if buyerBalance < highestBid.Amount {
		return nil, &AuctionBidderInsufficientSpiritStonesError{
			OrderID:  orderID,
			Required: highestBid.Amount,
			BidderID: highestBid.Bidder,
		}
	}

	sellerBalance, err := s.loadSpiritStonesForUpdate(ctx, tx, orderState.SellerUserID)
	if err != nil {
		return nil, err
	}

	buyerItems, err := s.loadInventoryItemsForUpdate(ctx, tx, highestBid.Bidder)
	if err != nil {
		return nil, err
	}
	buyerItems = append(buyerItems, orderState.Item)
	if err := s.updateInventoryItems(ctx, tx, highestBid.Bidder, buyerItems); err != nil {
		return nil, err
	}

	feeAmount := highestBid.Amount * 5 / 100
	sellerIncome := highestBid.Amount - feeAmount
	nextBuyerBalance := buyerBalance - highestBid.Amount
	nextSellerBalance := sellerBalance + sellerIncome
	if err := s.updateSpiritStones(ctx, tx, highestBid.Bidder, nextBuyerBalance); err != nil {
		return nil, err
	}
	if err := s.updateSpiritStones(ctx, tx, orderState.SellerUserID, nextSellerBalance); err != nil {
		return nil, err
	}

	if err := s.insertEconomyLogTx(ctx, tx, highestBid.Bidder, "auction_buy_bid", -highestBid.Amount, nextBuyerBalance, fmt.Sprintf("auction_order:%d", orderID)); err != nil {
		return nil, err
	}
	if err := s.insertEconomyLogTx(ctx, tx, orderState.SellerUserID, "auction_sell_bid", sellerIncome, nextSellerBalance, fmt.Sprintf("auction_order:%d", orderID)); err != nil {
		return nil, err
	}

	if err := s.updateOrderSettlement(ctx, tx, orderID, highestBid.Amount, feeAmount, sellerIncome); err != nil {
		return nil, err
	}
	if err := s.closeOrderWithStatus(ctx, tx, orderID, "sold", highestBid.Bidder); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit auction accept bid transaction: %w", err)
	}

	order, err := s.getOrderByID(ctx, orderID, userID)
	if err != nil {
		return nil, err
	}
	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &AuctionActionResult{
		Message:  "接受出价成功",
		Order:    order,
		Snapshot: snapshot,
	}, nil
}

func (s *AuctionService) Bid(ctx context.Context, userID uuid.UUID, orderID int64, amount int64) (*AuctionActionResult, error) {
	if amount <= 0 {
		return nil, &InvalidAuctionBidAmountError{Amount: amount}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin auction bid transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	orderState, err := s.loadOrderForUpdate(ctx, tx, orderID)
	if err != nil {
		return nil, err
	}
	if orderState.Status != "open" {
		return nil, &AuctionOrderInvalidStatusError{OrderID: orderID, Status: orderState.Status}
	}
	if orderState.ExpiresAt != nil && orderState.ExpiresAt.Before(time.Now().UTC()) {
		return nil, &AuctionOrderExpiredError{OrderID: orderID}
	}
	if orderState.SellerUserID == userID {
		return nil, &AuctionSelfPurchaseError{OrderID: orderID}
	}
	if amount < orderState.Price {
		return nil, &AuctionBidTooLowError{Amount: amount, RequiredMore: orderState.Price - 1}
	}

	currentHighest, err := s.loadHighestActiveBidForUpdate(ctx, tx, orderID)
	if err != nil {
		return nil, err
	}
	if currentHighest != nil && amount <= currentHighest.Amount {
		return nil, &AuctionBidTooLowError{Amount: amount, RequiredMore: currentHighest.Amount}
	}

	bidderBalance, err := s.loadSpiritStonesForUpdate(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	if bidderBalance < amount {
		return nil, &AuctionInsufficientSpiritStonesError{Required: amount, Current: bidderBalance}
	}

	const deactivateSQL = `
		UPDATE auction_bids
		SET status = 'outbid', updated_at = now()
		WHERE order_id = $1
		  AND status = 'active'
	`
	if _, err := tx.Exec(ctx, deactivateSQL, orderID); err != nil {
		return nil, fmt.Errorf("deactivate active auction bids: %w", err)
	}

	const insertSQL = `
		INSERT INTO auction_bids (order_id, bidder_user_id, amount, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'active', now(), now())
	`
	if _, err := tx.Exec(ctx, insertSQL, orderID, userID, amount); err != nil {
		return nil, fmt.Errorf("insert auction bid: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit auction bid transaction: %w", err)
	}

	order, err := s.getOrderByID(ctx, orderID, userID)
	if err != nil {
		return nil, err
	}
	return &AuctionActionResult{
		Message: "出价成功",
		Order:   order,
	}, nil
}

type auctionOrderState struct {
	ID           int64
	SellerUserID uuid.UUID
	Price        int64
	SellerIncome int64
	Status       string
	ExpiresAt    *time.Time
	Item         map[string]any
}

type auctionBidState struct {
	ID     int64
	Bidder uuid.UUID
	Amount int64
}

func (s *AuctionService) loadOrderForUpdate(ctx context.Context, tx pgx.Tx, orderID int64) (*auctionOrderState, error) {
	const query = `
		SELECT
			id,
			seller_user_id,
			price,
			seller_income,
			status,
			expires_at,
			item_payload
		FROM auction_orders
		WHERE id = $1
		FOR UPDATE
	`

	state := &auctionOrderState{}
	var expiresAt sql.NullTime
	var itemRaw []byte
	if err := tx.QueryRow(ctx, query, orderID).Scan(
		&state.ID,
		&state.SellerUserID,
		&state.Price,
		&state.SellerIncome,
		&state.Status,
		&expiresAt,
		&itemRaw,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &AuctionOrderNotFoundError{OrderID: orderID}
		}
		return nil, fmt.Errorf("load auction order for update: %w", err)
	}
	if expiresAt.Valid {
		ts := expiresAt.Time.UTC()
		state.ExpiresAt = &ts
	}
	if err := json.Unmarshal(itemRaw, &state.Item); err != nil || state.Item == nil {
		state.Item = map[string]any{}
	}
	return state, nil
}

func (s *AuctionService) loadHighestActiveBidForUpdate(ctx context.Context, tx pgx.Tx, orderID int64) (*auctionBidState, error) {
	const query = `
		SELECT id, bidder_user_id, amount
		FROM auction_bids
		WHERE order_id = $1
		  AND status = 'active'
		ORDER BY amount DESC, id DESC
		LIMIT 1
		FOR UPDATE
	`
	bid := &auctionBidState{}
	if err := tx.QueryRow(ctx, query, orderID).Scan(&bid.ID, &bid.Bidder, &bid.Amount); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("load highest active bid for update: %w", err)
	}
	return bid, nil
}

func (s *AuctionService) closeOrderWithStatus(ctx context.Context, tx pgx.Tx, orderID int64, status string, buyerUserID uuid.UUID) error {
	const soldSQL = `
		UPDATE auction_orders
		SET status = $2, buyer_user_id = $3, updated_at = now(), closed_at = now()
		WHERE id = $1
	`
	const closedSQL = `
		UPDATE auction_orders
		SET status = $2, updated_at = now(), closed_at = now()
		WHERE id = $1
	`

	if buyerUserID != uuid.Nil {
		if _, err := tx.Exec(ctx, soldSQL, orderID, status, buyerUserID); err != nil {
			return fmt.Errorf("update auction order status (sold): %w", err)
		}
		if status == "sold" {
			const markWinnerSQL = `
				UPDATE auction_bids
				SET status = 'won', updated_at = now()
				WHERE order_id = $1
				  AND bidder_user_id = $2
				  AND status = 'active'
			`
			if _, err := tx.Exec(ctx, markWinnerSQL, orderID, buyerUserID); err != nil {
				return fmt.Errorf("mark winning auction bid: %w", err)
			}
			const markOthersSQL = `
				UPDATE auction_bids
				SET status = 'lost', updated_at = now()
				WHERE order_id = $1
				  AND bidder_user_id <> $2
				  AND status = 'active'
			`
			if _, err := tx.Exec(ctx, markOthersSQL, orderID, buyerUserID); err != nil {
				return fmt.Errorf("mark losing auction bids: %w", err)
			}
		}
		return nil
	}

	if _, err := tx.Exec(ctx, closedSQL, orderID, status); err != nil {
		return fmt.Errorf("update auction order status: %w", err)
	}
	const markBidStatusSQL = `
		UPDATE auction_bids
		SET status = $2, updated_at = now()
		WHERE order_id = $1
		  AND status = 'active'
	`
	bidStatus := "closed"
	switch status {
	case "cancelled":
		bidStatus = "cancelled"
	case "expired":
		bidStatus = "expired"
	}
	if _, err := tx.Exec(ctx, markBidStatusSQL, orderID, bidStatus); err != nil {
		return fmt.Errorf("update auction bid status: %w", err)
	}
	return nil
}

func (s *AuctionService) updateOrderSettlement(ctx context.Context, tx pgx.Tx, orderID int64, price int64, feeAmount int64, sellerIncome int64) error {
	const query = `
		UPDATE auction_orders
		SET price = $2,
		    fee_rate = $3,
		    fee_amount = $4,
		    seller_income = $5,
		    updated_at = now()
		WHERE id = $1
	`
	if _, err := tx.Exec(ctx, query, orderID, price, auctionFeeRate, feeAmount, sellerIncome); err != nil {
		return fmt.Errorf("update auction order settlement: %w", err)
	}
	return nil
}

func (s *AuctionService) loadInventoryItemsForUpdate(ctx context.Context, tx pgx.Tx, userID uuid.UUID) ([]map[string]any, error) {
	const query = `
		SELECT COALESCE(items, '[]'::jsonb)
		FROM player_inventory_state
		WHERE user_id = $1
		FOR UPDATE
	`

	var itemsRaw []byte
	if err := tx.QueryRow(ctx, query, userID).Scan(&itemsRaw); err != nil {
		return nil, fmt.Errorf("load inventory items for auction: %w", err)
	}

	items := []map[string]any{}
	if err := json.Unmarshal(itemsRaw, &items); err != nil || items == nil {
		items = []map[string]any{}
	}
	return items, nil
}

func (s *AuctionService) updateInventoryItems(ctx context.Context, tx pgx.Tx, userID uuid.UUID, items []map[string]any) error {
	itemsJSON, err := json.Marshal(items)
	if err != nil {
		return fmt.Errorf("marshal auction inventory items: %w", err)
	}
	const updateSQL = `
		UPDATE player_inventory_state
		SET items = $2::jsonb, updated_at = now()
		WHERE user_id = $1
	`
	if _, err := tx.Exec(ctx, updateSQL, userID, string(itemsJSON)); err != nil {
		return fmt.Errorf("update auction inventory items: %w", err)
	}
	return nil
}

func (s *AuctionService) loadSpiritStonesForUpdate(ctx context.Context, tx pgx.Tx, userID uuid.UUID) (int64, error) {
	const query = `
		SELECT spirit_stones
		FROM player_resources
		WHERE user_id = $1
		FOR UPDATE
	`
	var balance int64
	if err := tx.QueryRow(ctx, query, userID).Scan(&balance); err != nil {
		return 0, fmt.Errorf("load spirit stones for auction: %w", err)
	}
	return balance, nil
}

func (s *AuctionService) updateSpiritStones(ctx context.Context, tx pgx.Tx, userID uuid.UUID, value int64) error {
	const updateSQL = `
		UPDATE player_resources
		SET spirit_stones = $2, updated_at = now()
		WHERE user_id = $1
	`
	if _, err := tx.Exec(ctx, updateSQL, userID, value); err != nil {
		return fmt.Errorf("update spirit stones for auction: %w", err)
	}
	return nil
}

func (s *AuctionService) insertEconomyLogTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID, changeType string, amount int64, balanceAfter int64, detail string) error {
	const insertSQL = `
		INSERT INTO economy_logs (user_id, currency, change_type, amount, balance_after, detail, occurred_at)
		VALUES ($1, 'spirit_stones', $2, $3, $4, $5, $6)
	`
	if _, err := tx.Exec(ctx, insertSQL, userID, changeType, amount, balanceAfter, detail, time.Now().UTC()); err != nil {
		return fmt.Errorf("insert auction economy log: %w", err)
	}
	return nil
}

func (s *AuctionService) getOrderByID(ctx context.Context, orderID int64, viewerID uuid.UUID) (*AuctionOrder, error) {
	const query = `
		SELECT
			ao.id,
			ao.seller_user_id::text,
			COALESCE(sp.player_name, ''),
			COALESCE(ao.buyer_user_id::text, ''),
			COALESCE(hb.amount, 0),
			COALESCE(hb.bidder_user_id, ''),
			ao.item_id,
			ao.item_payload,
			ao.price,
			ao.fee_rate,
			ao.fee_amount,
			ao.seller_income,
			ao.status,
			ao.expires_at,
			ao.created_at,
			ao.updated_at,
			ao.closed_at
		FROM auction_orders ao
		JOIN player_profiles sp ON sp.user_id = ao.seller_user_id
		LEFT JOIN LATERAL (
			SELECT
				ab.amount,
				ab.bidder_user_id::text AS bidder_user_id
			FROM auction_bids ab
			WHERE ab.order_id = ao.id
			  AND ab.status = 'active'
			ORDER BY ab.amount DESC, ab.id DESC
			LIMIT 1
		) hb ON true
		WHERE ao.id = $1
	`

	row := s.pool.QueryRow(ctx, query, orderID)
	order, err := scanAuctionOrder(row, viewerID)
	if err != nil {
		var notFound *AuctionOrderNotFoundError
		if errors.As(err, &notFound) {
			return nil, &AuctionOrderNotFoundError{OrderID: orderID}
		}
		return nil, err
	}
	return &order, nil
}

func scanAuctionOrder(scanner interface {
	Scan(dest ...any) error
}, viewerID uuid.UUID) (AuctionOrder, error) {
	var (
		order            AuctionOrder
		itemRaw          []byte
		expiresAt        sql.NullTime
		closedAt         sql.NullTime
		buyerUserIDRaw   string
		highestBidderRaw string
	)

	err := scanner.Scan(
		&order.ID,
		&order.SellerUserID,
		&order.SellerName,
		&buyerUserIDRaw,
		&order.HighestBid,
		&highestBidderRaw,
		&order.ItemID,
		&itemRaw,
		&order.Price,
		&order.FeeRate,
		&order.FeeAmount,
		&order.SellerIncome,
		&order.Status,
		&expiresAt,
		&order.CreatedAt,
		&order.UpdatedAt,
		&closedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AuctionOrder{}, &AuctionOrderNotFoundError{}
		}
		return AuctionOrder{}, fmt.Errorf("scan auction order row: %w", err)
	}

	if err := json.Unmarshal(itemRaw, &order.Item); err != nil || order.Item == nil {
		order.Item = map[string]any{}
	}
	if buyerUserIDRaw != "" {
		order.BuyerUserID = buyerUserIDRaw
	}
	if highestBidderRaw != "" {
		order.HighestBidderUserID = highestBidderRaw
	}
	if expiresAt.Valid {
		ts := expiresAt.Time.UTC()
		order.ExpiresAt = &ts
	}
	if closedAt.Valid {
		ts := closedAt.Time.UTC()
		order.ClosedAt = &ts
	}
	order.IsMine = order.SellerUserID == viewerID.String()
	return order, nil
}

func auctionNormalizeDuration(hours int) (time.Duration, error) {
	if hours == 0 {
		hours = auctionDefaultDurationHours
	}
	if _, ok := auctionAllowedDurations[hours]; !ok {
		return 0, &InvalidAuctionDurationError{DurationHours: hours}
	}
	return time.Duration(hours) * time.Hour, nil
}

func auctionFindItemIndex(items []map[string]any, itemID string) int {
	for i, item := range items {
		if auctionReadString(item["id"]) == itemID {
			return i
		}
	}
	return -1
}

func auctionReadString(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(value, 10)
	case int:
		return strconv.Itoa(value)
	default:
		if v == nil {
			return ""
		}
		return fmt.Sprintf("%v", v)
	}
}

func auctionIsTradableType(itemType string) bool {
	switch itemType {
	case "pill", "weapon", "head", "body", "legs", "feet", "shoulder", "hands", "wrist", "necklace", "ring1", "ring2", "belt", "artifact":
		return true
	default:
		return false
	}
}
