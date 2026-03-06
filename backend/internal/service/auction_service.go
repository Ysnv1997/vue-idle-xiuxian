package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
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
	ID           int64          `json:"id"`
	SellerUserID string         `json:"sellerUserId"`
	SellerName   string         `json:"sellerName"`
	BuyerUserID  string         `json:"buyerUserId,omitempty"`
	ItemID       string         `json:"itemId"`
	Item         map[string]any `json:"item"`
	Price        int64          `json:"price"`
	FeeRate      float64        `json:"feeRate"`
	FeeAmount    int64          `json:"feeAmount"`
	SellerIncome int64          `json:"sellerIncome"`
	Status       string         `json:"status"`
	Category     string         `json:"category"`
	SubCategory  string         `json:"subCategory,omitempty"`
	ExpiresAt    *time.Time     `json:"expiresAt,omitempty"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
	ClosedAt     *time.Time     `json:"closedAt,omitempty"`
	IsMine       bool           `json:"isMine"`
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
		inventoryState, err := s.loadAuctionInventoryStateForUpdate(ctx, tx, order.SellerUserID)
		if err != nil {
			return nil, err
		}
		auctionAppendOrderItem(inventoryState, order.Item)
		if err := s.updateAuctionInventoryState(ctx, tx, order.SellerUserID, inventoryState); err != nil {
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

func (s *AuctionService) List(ctx context.Context, userID uuid.UUID, limit int, offset int, category string, subCategory string) (*AuctionListResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	category = normalizeAuctionCategory(category)
	subCategory = normalizeAuctionSubCategory(category, subCategory)

	const query = `
		SELECT
			ao.id,
			ao.seller_user_id::text,
			COALESCE(sp.player_name, ''),
			COALESCE(ao.buyer_user_id::text, ''),
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
		WHERE ao.status = 'open'
		  AND (ao.expires_at IS NULL OR ao.expires_at > now())
		  AND (
			$3 = ''
			OR (
				CASE
					WHEN ao.item_payload->>'type' IN ('weapon', 'head', 'body', 'legs', 'feet', 'shoulder', 'hands', 'wrist', 'necklace', 'ring1', 'ring2', 'belt', 'artifact') THEN 'equipment'
					WHEN ao.item_payload->>'type' = 'herb' THEN 'herb'
					WHEN ao.item_payload->>'type' = 'pill' THEN 'pill'
					WHEN ao.item_payload->>'type' = 'pill_fragment' THEN 'pill_fragment'
					WHEN ao.item_payload->>'type' = 'pet' THEN 'pet'
					ELSE 'other'
				END
			) = $3
		  )
		  AND (
			$4 = ''
			OR (
				CASE
					WHEN ao.item_payload->>'type' IN ('weapon', 'head', 'body', 'legs', 'feet', 'shoulder', 'hands', 'wrist', 'necklace', 'ring1', 'ring2', 'belt', 'artifact') THEN ao.item_payload->>'type'
					ELSE ''
				END
			) = $4
		  )
		ORDER BY ao.created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := s.pool.Query(ctx, query, limit, offset, category, subCategory)
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

	inventoryState, err := s.loadAuctionInventoryStateForUpdate(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	item, err := auctionTakeTradableItem(inventoryState, itemID)
	if err != nil {
		return nil, err
	}
	if err := s.updateAuctionInventoryState(ctx, tx, userID, inventoryState); err != nil {
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

	inventoryState, err := s.loadAuctionInventoryStateForUpdate(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	auctionAppendOrderItem(inventoryState, orderState.Item)
	if err := s.updateAuctionInventoryState(ctx, tx, userID, inventoryState); err != nil {
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

	buyerInventoryState, err := s.loadAuctionInventoryStateForUpdate(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	auctionAppendOrderItem(buyerInventoryState, orderState.Item)
	if err := s.updateAuctionInventoryState(ctx, tx, userID, buyerInventoryState); err != nil {
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

type auctionOrderState struct {
	ID           int64
	SellerUserID uuid.UUID
	Price        int64
	SellerIncome int64
	Status       string
	ExpiresAt    *time.Time
	Item         map[string]any
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
		return nil
	}

	if _, err := tx.Exec(ctx, closedSQL, orderID, status); err != nil {
		return fmt.Errorf("update auction order status: %w", err)
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
		order          AuctionOrder
		itemRaw        []byte
		expiresAt      sql.NullTime
		closedAt       sql.NullTime
		buyerUserIDRaw string
	)

	err := scanner.Scan(
		&order.ID,
		&order.SellerUserID,
		&order.SellerName,
		&buyerUserIDRaw,
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
	if expiresAt.Valid {
		ts := expiresAt.Time.UTC()
		order.ExpiresAt = &ts
	}
	if closedAt.Valid {
		ts := closedAt.Time.UTC()
		order.ClosedAt = &ts
	}
	order.Category, order.SubCategory = auctionResolveOrderCategory(order.Item)
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
	case "pill", "pet", "weapon", "head", "body", "legs", "feet", "shoulder", "hands", "wrist", "necklace", "ring1", "ring2", "belt", "artifact":
		return true
	default:
		return false
	}
}

type auctionInventoryState struct {
	ActivePetID   string
	Herbs         []herbItem
	PillFragments map[string]int64
	PillRecipes   []string
	Items         []map[string]any
}

func (s *AuctionService) loadAuctionInventoryStateForUpdate(ctx context.Context, tx pgx.Tx, userID uuid.UUID) (*auctionInventoryState, error) {
	const query = `
		SELECT
			COALESCE(active_pet_id, ''),
			COALESCE(herbs, '[]'::jsonb),
			COALESCE(pill_fragments, '{}'::jsonb),
			COALESCE(pill_recipes, '[]'::jsonb),
			COALESCE(items, '[]'::jsonb)
		FROM player_inventory_state
		WHERE user_id = $1
		FOR UPDATE
	`

	var activePetID string
	var herbsRaw []byte
	var pillFragmentsRaw []byte
	var pillRecipesRaw []byte
	var itemsRaw []byte
	if err := tx.QueryRow(ctx, query, userID).Scan(&activePetID, &herbsRaw, &pillFragmentsRaw, &pillRecipesRaw, &itemsRaw); err != nil {
		return nil, fmt.Errorf("load auction inventory state: %w", err)
	}

	state := &auctionInventoryState{
		ActivePetID:   activePetID,
		Herbs:         []herbItem{},
		PillFragments: map[string]int64{},
		PillRecipes:   []string{},
		Items:         []map[string]any{},
	}
	if err := json.Unmarshal(herbsRaw, &state.Herbs); err != nil || state.Herbs == nil {
		state.Herbs = []herbItem{}
	}
	if err := json.Unmarshal(pillFragmentsRaw, &state.PillFragments); err != nil || state.PillFragments == nil {
		state.PillFragments = map[string]int64{}
	}
	if err := json.Unmarshal(pillRecipesRaw, &state.PillRecipes); err != nil || state.PillRecipes == nil {
		state.PillRecipes = []string{}
	}
	if err := json.Unmarshal(itemsRaw, &state.Items); err != nil || state.Items == nil {
		state.Items = []map[string]any{}
	}
	return state, nil
}

func (s *AuctionService) updateAuctionInventoryState(ctx context.Context, tx pgx.Tx, userID uuid.UUID, state *auctionInventoryState) error {
	herbsJSON, err := json.Marshal(state.Herbs)
	if err != nil {
		return fmt.Errorf("marshal auction herbs: %w", err)
	}
	pillFragmentsJSON, err := json.Marshal(state.PillFragments)
	if err != nil {
		return fmt.Errorf("marshal auction pill fragments: %w", err)
	}
	pillRecipesJSON, err := json.Marshal(state.PillRecipes)
	if err != nil {
		return fmt.Errorf("marshal auction pill recipes: %w", err)
	}
	itemsJSON, err := json.Marshal(state.Items)
	if err != nil {
		return fmt.Errorf("marshal auction items: %w", err)
	}

	const updateSQL = `
		UPDATE player_inventory_state
		SET herbs = $2::jsonb,
		    pill_fragments = $3::jsonb,
		    pill_recipes = $4::jsonb,
		    items = $5::jsonb,
		    updated_at = now()
		WHERE user_id = $1
	`
	if _, err := tx.Exec(ctx, updateSQL, userID, string(herbsJSON), string(pillFragmentsJSON), string(pillRecipesJSON), string(itemsJSON)); err != nil {
		return fmt.Errorf("update auction inventory state: %w", err)
	}
	return nil
}

func auctionTakeTradableItem(state *auctionInventoryState, itemID string) (map[string]any, error) {
	if herbID, herbQuality, ok := auctionParseHerbListingID(itemID); ok {
		index := auctionFindHerbIndex(state.Herbs, herbID, herbQuality)
		if index < 0 {
			return nil, &AuctionItemNotFoundError{ItemID: itemID}
		}
		herb := state.Herbs[index]
		state.Herbs = append(state.Herbs[:index], state.Herbs[index+1:]...)
		return map[string]any{
			"type":        "herb",
			"id":          herb.ID,
			"name":        herb.Name,
			"description": herb.Description,
			"baseValue":   herb.BaseValue,
			"category":    herb.Category,
			"chance":      herb.Chance,
			"quality":     herb.Quality,
			"value":       herb.Value,
		}, nil
	}

	if recipeID, ok := auctionParseFragmentListingID(itemID); ok {
		current := state.PillFragments[recipeID]
		if current <= 0 {
			return nil, &AuctionItemNotFoundError{ItemID: itemID}
		}
		if current == 1 {
			delete(state.PillFragments, recipeID)
		} else {
			state.PillFragments[recipeID] = current - 1
		}
		return map[string]any{
			"type":     "pill_fragment",
			"recipeId": recipeID,
			"name":     auctionRecipeNameByID(recipeID),
			"count":    int64(1),
		}, nil
	}

	index := auctionFindItemIndex(state.Items, itemID)
	if index < 0 {
		return nil, &AuctionItemNotFoundError{ItemID: itemID}
	}
	item := state.Items[index]
	itemType := auctionReadString(item["type"])
	if !auctionIsTradableType(itemType) {
		return nil, &AuctionItemNotTradableError{ItemID: itemID, ItemType: itemType}
	}
	if itemType == "pet" && state.ActivePetID != "" && state.ActivePetID == itemID {
		return nil, &AuctionItemNotTradableError{ItemID: itemID, ItemType: "pet_active"}
	}
	state.Items = append(state.Items[:index], state.Items[index+1:]...)
	return item, nil
}

func auctionAppendOrderItem(state *auctionInventoryState, item map[string]any) {
	itemType := auctionReadString(item["type"])
	switch itemType {
	case "herb":
		state.Herbs = append(state.Herbs, herbItem{
			ID:          auctionReadString(item["id"]),
			Name:        auctionReadString(item["name"]),
			Description: auctionReadString(item["description"]),
			BaseValue:   auctionReadInt64(item["baseValue"], 0),
			Category:    auctionReadString(item["category"]),
			Chance:      auctionReadFloat(item["chance"], 0),
			Quality:     auctionReadString(item["quality"]),
			Value:       auctionReadInt64(item["value"], 0),
		})
	case "pill_fragment":
		recipeID := auctionReadString(item["recipeId"])
		if recipeID == "" {
			return
		}
		count := auctionReadInt64(item["count"], 1)
		if count <= 0 {
			count = 1
		}
		state.PillFragments[recipeID] += count
	default:
		state.Items = append(state.Items, item)
	}
}

func normalizeAuctionCategory(category string) string {
	switch strings.TrimSpace(strings.ToLower(category)) {
	case "equipment", "herb", "pill", "pill_fragment", "pet":
		return strings.TrimSpace(strings.ToLower(category))
	default:
		return ""
	}
}

func normalizeAuctionSubCategory(category string, subCategory string) string {
	cleaned := strings.TrimSpace(strings.ToLower(subCategory))
	if category != "equipment" || cleaned == "" {
		return ""
	}
	if inventoryIsEquipmentType(cleaned) {
		return cleaned
	}
	return ""
}

func auctionResolveOrderCategory(item map[string]any) (string, string) {
	itemType := auctionReadString(item["type"])
	if inventoryIsEquipmentType(itemType) {
		return "equipment", itemType
	}
	switch itemType {
	case "herb":
		return "herb", ""
	case "pill":
		return "pill", ""
	case "pill_fragment":
		return "pill_fragment", ""
	case "pet":
		return "pet", ""
	default:
		return "other", ""
	}
}

func auctionFindHerbIndex(herbs []herbItem, herbID string, herbQuality string) int {
	for i, herb := range herbs {
		if herb.ID == herbID && herb.Quality == herbQuality {
			return i
		}
	}
	return -1
}

func auctionParseHerbListingID(itemID string) (string, string, bool) {
	const prefix = "herb:"
	if !strings.HasPrefix(itemID, prefix) {
		return "", "", false
	}
	parts := strings.SplitN(strings.TrimPrefix(itemID, prefix), ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func auctionParseFragmentListingID(itemID string) (string, bool) {
	const prefix = "fragment:"
	if !strings.HasPrefix(itemID, prefix) {
		return "", false
	}
	recipeID := strings.TrimSpace(strings.TrimPrefix(itemID, prefix))
	if recipeID == "" {
		return "", false
	}
	return recipeID, true
}

func auctionRecipeNameByID(recipeID string) string {
	for _, recipe := range pillRecipeDefinitions {
		if recipe.ID == recipeID {
			return recipe.Name
		}
	}
	return recipeID
}

func auctionReadInt64(v any, fallback int64) int64 {
	switch value := v.(type) {
	case int64:
		return value
	case int:
		return int64(value)
	case float64:
		return int64(value)
	case json.Number:
		parsed, err := value.Int64()
		if err != nil {
			return fallback
		}
		return parsed
	default:
		return fallback
	}
}

func auctionReadFloat(v any, fallback float64) float64 {
	switch value := v.(type) {
	case float64:
		return value
	case int64:
		return float64(value)
	case int:
		return float64(value)
	case json.Number:
		parsed, err := value.Float64()
		if err != nil {
			return fallback
		}
		return parsed
	default:
		return fallback
	}
}
