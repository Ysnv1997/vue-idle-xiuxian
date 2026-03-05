package service

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kowming/vue-idle-xiuxian/backend/internal/repository"
)

const (
	rechargeStatusPending   = "pending"
	rechargeStatusPaid      = "paid"
	rechargeStatusFailed    = "failed"
	rechargeStatusCancelled = "cancelled"
)

const pgUniqueViolationCode = "23505"

type RechargeService struct {
	pool       *pgxpool.Pool
	userRepo   *repository.UserRepository
	epay       RechargeEPayConfig
	httpClient *http.Client
}

type RechargeEPayConfig struct {
	PID       string
	Key       string
	BaseURL   string
	NotifyURL string
	ReturnURL string
}

func NewRechargeService(
	pool *pgxpool.Pool,
	userRepo *repository.UserRepository,
	epay RechargeEPayConfig,
) *RechargeService {
	epay.BaseURL = strings.TrimRight(strings.TrimSpace(epay.BaseURL), "/")
	return &RechargeService{
		pool:     pool,
		userRepo: userRepo,
		epay: RechargeEPayConfig{
			PID:       strings.TrimSpace(epay.PID),
			Key:       strings.TrimSpace(epay.Key),
			BaseURL:   epay.BaseURL,
			NotifyURL: strings.TrimSpace(epay.NotifyURL),
			ReturnURL: strings.TrimSpace(epay.ReturnURL),
		},
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

type RechargeProduct struct {
	ID          int64   `json:"id"`
	Code        string  `json:"code"`
	Credit      int     `json:"creditAmount"`
	SpiritStone int64   `json:"spiritStones"`
	BonusRate   float64 `json:"bonusRate"`
	Enabled     bool    `json:"enabled"`
}

type RechargeProductListResult struct {
	Products []RechargeProduct `json:"products"`
}

type RechargeOrder struct {
	ID              int64      `json:"id"`
	UserID          uuid.UUID  `json:"-"`
	ProductCode     string     `json:"productCode"`
	CreditAmount    int        `json:"creditAmount"`
	SpiritStones    int64      `json:"spiritStones"`
	ExternalOrderID string     `json:"externalOrderId,omitempty"`
	Status          string     `json:"status"`
	IdempotencyKey  string     `json:"idempotencyKey,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	PaidAt          *time.Time `json:"paidAt,omitempty"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type RechargeOrderListResult struct {
	Orders []RechargeOrder `json:"orders"`
}

type RechargeCreateInput struct {
	ProductCode    string
	IdempotencyKey string
}

type RechargeCreateResult struct {
	Order       *RechargeOrder             `json:"order"`
	CheckoutURL string                     `json:"checkoutUrl,omitempty"`
	Snapshot    *repository.PlayerSnapshot `json:"snapshot,omitempty"`
}

type RechargeActionResult struct {
	Message  string                     `json:"message"`
	Credited bool                       `json:"credited"`
	Order    *RechargeOrder             `json:"order,omitempty"`
	Snapshot *repository.PlayerSnapshot `json:"snapshot,omitempty"`
}

type RechargeCallbackResult struct {
	Accepted       bool           `json:"accepted"`
	SignatureValid bool           `json:"signatureValid"`
	Credited       bool           `json:"credited"`
	Order          *RechargeOrder `json:"order,omitempty"`
}

type RechargeProductNotFoundError struct {
	ProductCode string
}

func (e *RechargeProductNotFoundError) Error() string {
	return fmt.Sprintf("recharge product not found: %s", e.ProductCode)
}

type RechargeProductDisabledError struct {
	ProductCode string
}

func (e *RechargeProductDisabledError) Error() string {
	return fmt.Sprintf("recharge product disabled: %s", e.ProductCode)
}

type RechargeOrderNotFoundError struct {
	OrderID         int64
	ExternalOrderID string
}

func (e *RechargeOrderNotFoundError) Error() string {
	if e.OrderID > 0 {
		return fmt.Sprintf("recharge order not found: %d", e.OrderID)
	}
	return fmt.Sprintf("recharge order not found by external id: %s", e.ExternalOrderID)
}

type RechargeOrderForbiddenError struct {
	OrderID int64
}

func (e *RechargeOrderForbiddenError) Error() string {
	return fmt.Sprintf("recharge order forbidden: %d", e.OrderID)
}

type RechargeInvalidCallbackSignatureError struct{}

func (e *RechargeInvalidCallbackSignatureError) Error() string {
	return "invalid recharge callback signature"
}

type RechargeInvalidCallbackPayloadError struct {
	Reason string
}

func (e *RechargeInvalidCallbackPayloadError) Error() string {
	return fmt.Sprintf("invalid recharge callback payload: %s", e.Reason)
}

type RechargeIdempotencyConflictError struct {
	IdempotencyKey string
}

func (e *RechargeIdempotencyConflictError) Error() string {
	return fmt.Sprintf("recharge idempotency key conflict: %s", e.IdempotencyKey)
}

type RechargeProviderConfigError struct {
	Reason string
}

func (e *RechargeProviderConfigError) Error() string {
	if strings.TrimSpace(e.Reason) == "" {
		return "recharge provider config invalid"
	}
	return fmt.Sprintf("recharge provider config invalid: %s", e.Reason)
}

type RechargeProviderRequestError struct {
	Reason string
}

func (e *RechargeProviderRequestError) Error() string {
	if strings.TrimSpace(e.Reason) == "" {
		return "recharge provider request failed"
	}
	return fmt.Sprintf("recharge provider request failed: %s", e.Reason)
}

func (s *RechargeService) ListProducts(ctx context.Context, includeDisabled bool) (*RechargeProductListResult, error) {
	const query = `
		SELECT id, code, credit_amount, spirit_stones, bonus_rate, enabled
		FROM recharge_products
		WHERE ($1 OR enabled = TRUE)
		ORDER BY credit_amount ASC, id ASC
	`

	rows, err := s.pool.Query(ctx, query, includeDisabled)
	if err != nil {
		return nil, fmt.Errorf("query recharge products: %w", err)
	}
	defer rows.Close()

	products := make([]RechargeProduct, 0, 8)
	for rows.Next() {
		product := RechargeProduct{}
		if err := rows.Scan(
			&product.ID,
			&product.Code,
			&product.Credit,
			&product.SpiritStone,
			&product.BonusRate,
			&product.Enabled,
		); err != nil {
			return nil, fmt.Errorf("scan recharge product row: %w", err)
		}
		products = append(products, product)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate recharge products rows: %w", rows.Err())
	}

	return &RechargeProductListResult{Products: products}, nil
}

func (s *RechargeService) ListOrders(ctx context.Context, userID uuid.UUID, limit int) (*RechargeOrderListResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	const query = `
		SELECT
			id,
			user_id,
			product_code,
			credit_amount,
			spirit_stones,
			COALESCE(external_order_id, ''),
			status,
			idempotency_key,
			created_at,
			paid_at,
			updated_at
		FROM recharge_orders
		WHERE user_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`

	rows, err := s.pool.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("query recharge orders: %w", err)
	}
	defer rows.Close()

	orders := make([]RechargeOrder, 0, limit)
	for rows.Next() {
		order, scanErr := scanRechargeOrder(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		orders = append(orders, order)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate recharge orders rows: %w", rows.Err())
	}

	return &RechargeOrderListResult{Orders: orders}, nil
}

func (s *RechargeService) CreateOrder(ctx context.Context, userID uuid.UUID, input RechargeCreateInput) (*RechargeCreateResult, error) {
	productCode := strings.TrimSpace(input.ProductCode)
	if productCode == "" {
		return nil, &RechargeProductNotFoundError{ProductCode: productCode}
	}

	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = uuid.NewString()
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin recharge create transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	product, err := loadRechargeProductForCreateTx(ctx, tx, productCode)
	if err != nil {
		return nil, err
	}
	if !product.Enabled {
		return nil, &RechargeProductDisabledError{ProductCode: productCode}
	}

	order, err := insertRechargeOrderTx(ctx, tx, userID, product, idempotencyKey)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolationCode {
			existingOrder, loadErr := loadRechargeOrderByIdempotencyTx(ctx, tx, idempotencyKey)
			if loadErr != nil {
				return nil, loadErr
			}
			if existingOrder.UserID != userID {
				return nil, &RechargeIdempotencyConflictError{IdempotencyKey: idempotencyKey}
			}

			snapshot, snapErr := s.userRepo.GetSnapshot(ctx, userID)
			if snapErr != nil {
				return nil, snapErr
			}

			checkoutURL := ""
			if existingOrder.Status == rechargeStatusPending {
				checkoutURL, err = s.createEPayCheckout(ctx, existingOrder)
				if err != nil {
					return nil, err
				}
			}

			return &RechargeCreateResult{
				Order:       existingOrder,
				CheckoutURL: checkoutURL,
				Snapshot:    snapshot,
			}, nil
		}
		return nil, err
	}

	if order.ExternalOrderID == "" {
		order.ExternalOrderID = fmt.Sprintf("rx_%d_%s", order.ID, uuid.NewString()[:8])
		const updateSQL = `
			UPDATE recharge_orders
			SET external_order_id = $2, updated_at = now()
			WHERE id = $1
			RETURNING updated_at
		`
		if updateErr := tx.QueryRow(ctx, updateSQL, order.ID, order.ExternalOrderID).Scan(&order.UpdatedAt); updateErr != nil {
			return nil, fmt.Errorf("update recharge external order id: %w", updateErr)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit recharge create transaction: %w", err)
	}

	checkoutURL, err := s.createEPayCheckout(ctx, order)
	if err != nil {
		return nil, err
	}

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &RechargeCreateResult{
		Order:       order,
		CheckoutURL: checkoutURL,
		Snapshot:    snapshot,
	}, nil
}

func (s *RechargeService) HandleCreditLinuxDoCallback(
	ctx context.Context,
	params map[string]string,
) (*RechargeCallbackResult, error) {
	normalizedParams := normalizeCallbackParams(params)
	externalOrderID := firstNonEmptyString(
		normalizedParams["out_trade_no"],
		normalizedParams["outTradeNo"],
		normalizedParams["external_order_id"],
		normalizedParams["externalOrderId"],
	)
	signatureValid := s.validateEPayCallbackSignature(normalizedParams)

	payloadBytes, marshalErr := json.Marshal(normalizedParams)
	if marshalErr != nil {
		payloadBytes = []byte("{}")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin recharge callback transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const insertCallbackSQL = `
		INSERT INTO recharge_callbacks (external_order_id, payload, signature_valid, received_at)
		VALUES (NULLIF($1, ''), $2::jsonb, $3, now())
	`
	if _, err := tx.Exec(ctx, insertCallbackSQL, externalOrderID, string(payloadBytes), signatureValid); err != nil {
		return nil, fmt.Errorf("insert recharge callback: %w", err)
	}

	if !signatureValid {
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit recharge callback with invalid signature: %w", err)
		}
		return nil, &RechargeInvalidCallbackSignatureError{}
	}

	if externalOrderID == "" {
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit recharge callback without external order id: %w", err)
		}
		return nil, &RechargeInvalidCallbackPayloadError{Reason: "missing out_trade_no"}
	}

	if strings.TrimSpace(normalizedParams["type"]) != "" && strings.ToLower(strings.TrimSpace(normalizedParams["type"])) != "epay" {
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit recharge callback with invalid type: %w", err)
		}
		return nil, &RechargeInvalidCallbackPayloadError{Reason: "type must be epay"}
	}

	if s.epay.PID != "" && strings.TrimSpace(normalizedParams["pid"]) != s.epay.PID {
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit recharge callback with pid mismatch: %w", err)
		}
		return nil, &RechargeInvalidCallbackPayloadError{Reason: "pid mismatch"}
	}

	tradeStatus := strings.ToUpper(strings.TrimSpace(normalizedParams["trade_status"]))
	if tradeStatus != "TRADE_SUCCESS" {
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit recharge callback with invalid trade_status: %w", err)
		}
		return nil, &RechargeInvalidCallbackPayloadError{Reason: "trade_status must be TRADE_SUCCESS"}
	}

	order, err := loadRechargeOrderForUpdateByExternalIDTx(ctx, tx, externalOrderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if commitErr := tx.Commit(ctx); commitErr != nil {
				return nil, fmt.Errorf("commit recharge callback unknown order: %w", commitErr)
			}
			return nil, &RechargeOrderNotFoundError{ExternalOrderID: externalOrderID}
		}
		return nil, err
	}

	callbackMoneyRaw := strings.TrimSpace(normalizedParams["money"])
	if callbackMoneyRaw != "" {
		callbackMoneyCents, parseErr := parseMoneyToCents(callbackMoneyRaw)
		if parseErr != nil {
			if err := tx.Commit(ctx); err != nil {
				return nil, fmt.Errorf("commit recharge callback with invalid money: %w", err)
			}
			return nil, &RechargeInvalidCallbackPayloadError{Reason: "invalid money"}
		}
		expectedMoneyCents := int64(order.CreditAmount) * 100
		if callbackMoneyCents != expectedMoneyCents {
			if err := tx.Commit(ctx); err != nil {
				return nil, fmt.Errorf("commit recharge callback with money mismatch: %w", err)
			}
			return nil, &RechargeInvalidCallbackPayloadError{Reason: "money mismatch"}
		}
	}

	credited, err := markRechargeOrderPaidTx(ctx, tx, order, time.Now().UTC())
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit recharge callback transaction: %w", err)
	}

	return &RechargeCallbackResult{
		Accepted:       true,
		SignatureValid: true,
		Credited:       credited,
		Order:          order,
	}, nil
}

func (s *RechargeService) MockMarkPaid(ctx context.Context, userID uuid.UUID, orderID int64) (*RechargeActionResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin recharge mock paid transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	order, err := loadRechargeOrderForUpdateByIDTx(ctx, tx, orderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &RechargeOrderNotFoundError{OrderID: orderID}
		}
		return nil, err
	}
	if order.UserID != userID {
		return nil, &RechargeOrderForbiddenError{OrderID: orderID}
	}

	credited, err := markRechargeOrderPaidTx(ctx, tx, order, time.Now().UTC())
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit recharge mock paid transaction: %w", err)
	}

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	message := "订单已是支付成功状态"
	if credited {
		message = "模拟支付成功，灵石已到账"
	}

	return &RechargeActionResult{
		Message:  message,
		Credited: credited,
		Order:    order,
		Snapshot: snapshot,
	}, nil
}

func (s *RechargeService) SyncOrder(ctx context.Context, userID uuid.UUID, orderID int64) (*RechargeActionResult, error) {
	if orderID <= 0 {
		return nil, &RechargeOrderNotFoundError{OrderID: orderID}
	}

	order, err := s.loadOrderByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.UserID != userID {
		return nil, &RechargeOrderForbiddenError{OrderID: orderID}
	}
	if order.Status == rechargeStatusPaid {
		snapshot, snapErr := s.userRepo.GetSnapshot(ctx, userID)
		if snapErr != nil {
			return nil, snapErr
		}
		return &RechargeActionResult{
			Message:  "订单已支付",
			Credited: false,
			Order:    order,
			Snapshot: snapshot,
		}, nil
	}

	queryResult, err := s.queryEPayOrder(ctx, order.ExternalOrderID)
	if err != nil {
		return nil, err
	}
	if queryResult.Status != 1 {
		snapshot, snapErr := s.userRepo.GetSnapshot(ctx, userID)
		if snapErr != nil {
			return nil, snapErr
		}
		return &RechargeActionResult{
			Message:  "订单仍在处理中",
			Credited: false,
			Order:    order,
			Snapshot: snapshot,
		}, nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin recharge sync transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	lockedOrder, err := loadRechargeOrderForUpdateByIDTx(ctx, tx, orderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &RechargeOrderNotFoundError{OrderID: orderID}
		}
		return nil, err
	}
	if lockedOrder.UserID != userID {
		return nil, &RechargeOrderForbiddenError{OrderID: orderID}
	}

	credited, err := markRechargeOrderPaidTx(ctx, tx, lockedOrder, time.Now().UTC())
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit recharge sync transaction: %w", err)
	}

	snapshot, err := s.userRepo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	message := "订单状态已同步，仍未支付"
	if credited {
		message = "订单支付成功，灵石已到账"
	}
	return &RechargeActionResult{
		Message:  message,
		Credited: credited,
		Order:    lockedOrder,
		Snapshot: snapshot,
	}, nil
}

func (s *RechargeService) createEPayCheckout(ctx context.Context, order *RechargeOrder) (string, error) {
	if order == nil {
		return "", &RechargeProviderRequestError{Reason: "empty order"}
	}
	if strings.TrimSpace(s.epay.PID) == "" {
		return "", &RechargeProviderConfigError{Reason: "RECHARGE_EPAY_PID is empty"}
	}
	if strings.TrimSpace(s.epay.Key) == "" {
		return "", &RechargeProviderConfigError{Reason: "RECHARGE_EPAY_KEY is empty"}
	}
	if strings.TrimSpace(s.epay.BaseURL) == "" {
		return "", &RechargeProviderConfigError{Reason: "RECHARGE_EPAY_BASE_URL is empty"}
	}

	params := map[string]string{
		"pid":          s.epay.PID,
		"type":         "epay",
		"out_trade_no": order.ExternalOrderID,
		"name":         truncateRechargeName(fmt.Sprintf("充值-%s", order.ProductCode), 64),
		"money":        formatMoneyFromCents(int64(order.CreditAmount) * 100),
	}
	if strings.TrimSpace(s.epay.NotifyURL) != "" {
		params["notify_url"] = strings.TrimSpace(s.epay.NotifyURL)
	}
	if strings.TrimSpace(s.epay.ReturnURL) != "" {
		params["return_url"] = strings.TrimSpace(s.epay.ReturnURL)
	}
	params["sign"] = buildEPaySign(params, s.epay.Key)
	params["sign_type"] = "MD5"

	form := url.Values{}
	for key, value := range params {
		form.Set(key, value)
	}

	requestURL := strings.TrimRight(s.epay.BaseURL, "/") + "/pay/submit.php"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", &RechargeProviderRequestError{Reason: err.Error()}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json, text/html;q=0.9, */*;q=0.8")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", &RechargeProviderRequestError{Reason: err.Error()}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	location := strings.TrimSpace(resp.Header.Get("Location"))
	if location != "" {
		if parsed, parseErr := url.Parse(location); parseErr == nil && !parsed.IsAbs() {
			baseParsed, baseErr := url.Parse(s.epay.BaseURL)
			if baseErr == nil {
				location = baseParsed.ResolveReference(parsed).String()
			}
		}
		return location, nil
	}

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return "", &RechargeProviderRequestError{Reason: "missing redirect location"}
	}

	if len(body) > 0 {
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err == nil {
			if message := firstNonEmptyString(
				readStringFromMap(payload, "error_msg"),
				readStringFromMap(payload, "msg"),
				readStringFromMap(payload, "message"),
			); message != "" {
				return "", &RechargeProviderRequestError{Reason: message}
			}
		}
	}

	return "", &RechargeProviderRequestError{Reason: fmt.Sprintf("unexpected provider response status=%d body=%s", resp.StatusCode, string(body))}
}

type epayQueryOrderResult struct {
	Code   int
	Msg    string
	Status int
}

func (s *RechargeService) queryEPayOrder(ctx context.Context, outTradeNo string) (*epayQueryOrderResult, error) {
	if strings.TrimSpace(outTradeNo) == "" {
		return nil, &RechargeProviderRequestError{Reason: "missing out_trade_no"}
	}
	if strings.TrimSpace(s.epay.PID) == "" {
		return nil, &RechargeProviderConfigError{Reason: "RECHARGE_EPAY_PID is empty"}
	}
	if strings.TrimSpace(s.epay.Key) == "" {
		return nil, &RechargeProviderConfigError{Reason: "RECHARGE_EPAY_KEY is empty"}
	}
	if strings.TrimSpace(s.epay.BaseURL) == "" {
		return nil, &RechargeProviderConfigError{Reason: "RECHARGE_EPAY_BASE_URL is empty"}
	}

	queryURL := strings.TrimRight(s.epay.BaseURL, "/") + "/api.php"
	params := url.Values{}
	params.Set("act", "order")
	params.Set("pid", s.epay.PID)
	params.Set("key", s.epay.Key)
	params.Set("out_trade_no", outTradeNo)
	queryURL += "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, queryURL, nil)
	if err != nil {
		return nil, &RechargeProviderRequestError{Reason: err.Error()}
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, &RechargeProviderRequestError{Reason: err.Error()}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	payload := map[string]any{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, &RechargeProviderRequestError{Reason: fmt.Sprintf("invalid query response: %s", string(body))}
		}
	}

	result := &epayQueryOrderResult{
		Code:   parseAnyInt(payload["code"]),
		Msg:    firstNonEmptyString(readStringFromMap(payload, "msg"), readStringFromMap(payload, "message")),
		Status: parseAnyInt(payload["status"]),
	}

	if resp.StatusCode >= 400 {
		if result.Msg == "" {
			result.Msg = fmt.Sprintf("query order failed with status %d", resp.StatusCode)
		}
		return result, &RechargeProviderRequestError{Reason: result.Msg}
	}

	return result, nil
}

func parseAnyInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

func (s *RechargeService) loadOrderByID(ctx context.Context, orderID int64) (*RechargeOrder, error) {
	const query = `
		SELECT
			id,
			user_id,
			product_code,
			credit_amount,
			spirit_stones,
			COALESCE(external_order_id, ''),
			status,
			idempotency_key,
			created_at,
			paid_at,
			updated_at
		FROM recharge_orders
		WHERE id = $1
	`
	row := s.pool.QueryRow(ctx, query, orderID)
	order, err := scanRechargeOrder(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &RechargeOrderNotFoundError{OrderID: orderID}
		}
		return nil, err
	}
	return &order, nil
}

func (s *RechargeService) validateEPayCallbackSignature(params map[string]string) bool {
	if strings.TrimSpace(s.epay.Key) == "" {
		return false
	}
	received := strings.ToLower(strings.TrimSpace(params["sign"]))
	if received == "" {
		return false
	}
	expected := buildEPaySign(params, s.epay.Key)
	return received == expected
}

func buildEPaySign(params map[string]string, key string) string {
	payload := buildEPayPayloadForSign(params)
	sum := md5.Sum([]byte(payload + key))
	return fmt.Sprintf("%x", sum)
}

func buildEPayPayloadForSign(params map[string]string) string {
	if len(params) == 0 {
		return ""
	}

	keys := make([]string, 0, len(params))
	for key, value := range params {
		if key == "sign" || key == "sign_type" {
			continue
		}
		if strings.TrimSpace(value) == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, params[key]))
	}
	return strings.Join(parts, "&")
}

func normalizeCallbackParams(input map[string]string) map[string]string {
	if len(input) == 0 {
		return map[string]string{}
	}

	normalized := make(map[string]string, len(input))
	for key, value := range input {
		normalized[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return normalized
}

func parseMoneyToCents(raw string) (int64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, fmt.Errorf("money is empty")
	}
	if strings.HasPrefix(value, "-") {
		return 0, fmt.Errorf("money must be positive")
	}

	parts := strings.Split(value, ".")
	if len(parts) > 2 {
		return 0, fmt.Errorf("invalid money format")
	}

	intPartRaw := parts[0]
	if intPartRaw == "" {
		intPartRaw = "0"
	}
	intPart, err := strconv.ParseInt(intPartRaw, 10, 64)
	if err != nil || intPart < 0 {
		return 0, fmt.Errorf("invalid integer part")
	}

	decPart := int64(0)
	if len(parts) == 2 {
		decRaw := parts[1]
		if len(decRaw) > 2 {
			return 0, fmt.Errorf("decimal part exceeds 2 digits")
		}
		for len(decRaw) < 2 {
			decRaw += "0"
		}
		decPart, err = strconv.ParseInt(decRaw, 10, 64)
		if err != nil || decPart < 0 {
			return 0, fmt.Errorf("invalid decimal part")
		}
	}

	return intPart*100 + decPart, nil
}

func formatMoneyFromCents(cents int64) string {
	if cents%100 == 0 {
		return strconv.FormatInt(cents/100, 10)
	}
	if cents%10 == 0 {
		return fmt.Sprintf("%d.%d", cents/100, (cents%100)/10)
	}
	return fmt.Sprintf("%d.%02d", cents/100, cents%100)
}

func truncateRechargeName(value string, maxLen int) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "充值订单"
	}
	runes := []rune(trimmed)
	if maxLen <= 0 || len(runes) <= maxLen {
		return trimmed
	}
	return string(runes[:maxLen])
}

func readStringFromMap(payload map[string]any, key string) string {
	raw, ok := payload[key]
	if !ok || raw == nil {
		return ""
	}
	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value)
	case float64:
		return strings.TrimSpace(strconv.FormatFloat(value, 'f', -1, 64))
	case int64:
		return strings.TrimSpace(strconv.FormatInt(value, 10))
	case int:
		return strings.TrimSpace(strconv.Itoa(value))
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", value))
	}
}

func loadRechargeProductForCreateTx(ctx context.Context, tx pgx.Tx, code string) (*RechargeProduct, error) {
	const query = `
		SELECT id, code, credit_amount, spirit_stones, bonus_rate, enabled
		FROM recharge_products
		WHERE code = $1
	`

	product := &RechargeProduct{}
	if err := tx.QueryRow(ctx, query, code).Scan(
		&product.ID,
		&product.Code,
		&product.Credit,
		&product.SpiritStone,
		&product.BonusRate,
		&product.Enabled,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &RechargeProductNotFoundError{ProductCode: code}
		}
		return nil, fmt.Errorf("load recharge product: %w", err)
	}
	return product, nil
}

func insertRechargeOrderTx(
	ctx context.Context,
	tx pgx.Tx,
	userID uuid.UUID,
	product *RechargeProduct,
	idempotencyKey string,
) (*RechargeOrder, error) {
	const insertSQL = `
		INSERT INTO recharge_orders (
			user_id,
			product_code,
			credit_amount,
			spirit_stones,
			status,
			idempotency_key,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, now(), now())
		RETURNING
			id,
			user_id,
			product_code,
			credit_amount,
			spirit_stones,
			COALESCE(external_order_id, ''),
			status,
			idempotency_key,
			created_at,
			paid_at,
			updated_at
	`

	row := tx.QueryRow(
		ctx,
		insertSQL,
		userID,
		product.Code,
		product.Credit,
		product.SpiritStone,
		rechargeStatusPending,
		idempotencyKey,
	)
	order, err := scanRechargeOrder(row)
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func loadRechargeOrderByIdempotencyTx(ctx context.Context, tx pgx.Tx, idempotencyKey string) (*RechargeOrder, error) {
	const query = `
		SELECT
			id,
			user_id,
			product_code,
			credit_amount,
			spirit_stones,
			COALESCE(external_order_id, ''),
			status,
			idempotency_key,
			created_at,
			paid_at,
			updated_at
		FROM recharge_orders
		WHERE idempotency_key = $1
	`
	row := tx.QueryRow(ctx, query, idempotencyKey)
	order, err := scanRechargeOrder(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &RechargeOrderNotFoundError{}
		}
		return nil, err
	}
	return &order, nil
}

func loadRechargeOrderForUpdateByExternalIDTx(ctx context.Context, tx pgx.Tx, externalOrderID string) (*RechargeOrder, error) {
	const query = `
		SELECT
			id,
			user_id,
			product_code,
			credit_amount,
			spirit_stones,
			COALESCE(external_order_id, ''),
			status,
			idempotency_key,
			created_at,
			paid_at,
			updated_at
		FROM recharge_orders
		WHERE external_order_id = $1
		FOR UPDATE
	`
	row := tx.QueryRow(ctx, query, externalOrderID)
	order, err := scanRechargeOrder(row)
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func loadRechargeOrderForUpdateByIDTx(ctx context.Context, tx pgx.Tx, orderID int64) (*RechargeOrder, error) {
	const query = `
		SELECT
			id,
			user_id,
			product_code,
			credit_amount,
			spirit_stones,
			COALESCE(external_order_id, ''),
			status,
			idempotency_key,
			created_at,
			paid_at,
			updated_at
		FROM recharge_orders
		WHERE id = $1
		FOR UPDATE
	`
	row := tx.QueryRow(ctx, query, orderID)
	order, err := scanRechargeOrder(row)
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func markRechargeOrderPaidTx(ctx context.Context, tx pgx.Tx, order *RechargeOrder, paidAt time.Time) (bool, error) {
	if order == nil {
		return false, fmt.Errorf("mark recharge order paid: empty order")
	}
	if order.Status == rechargeStatusPaid {
		return false, nil
	}

	currentBalance, err := loadRechargeSpiritStonesForUpdateTx(ctx, tx, order.UserID)
	if err != nil {
		return false, err
	}
	nextBalance := currentBalance + order.SpiritStones

	if err := updateRechargeSpiritStonesTx(ctx, tx, order.UserID, nextBalance); err != nil {
		return false, err
	}
	if err := insertRechargeEconomyLogTx(ctx, tx, order.UserID, "recharge_paid", order.SpiritStones, nextBalance, fmt.Sprintf("recharge_order:%d", order.ID)); err != nil {
		return false, err
	}

	if paidAt.IsZero() {
		paidAt = time.Now().UTC()
	}

	const updateOrderSQL = `
		UPDATE recharge_orders
		SET status = $2, paid_at = COALESCE(paid_at, $3), updated_at = now()
		WHERE id = $1
		RETURNING paid_at, updated_at
	`
	var paidAtValue sql.NullTime
	if err := tx.QueryRow(ctx, updateOrderSQL, order.ID, rechargeStatusPaid, paidAt).Scan(&paidAtValue, &order.UpdatedAt); err != nil {
		return false, fmt.Errorf("update recharge order paid status: %w", err)
	}

	order.Status = rechargeStatusPaid
	if paidAtValue.Valid {
		ts := paidAtValue.Time.UTC()
		order.PaidAt = &ts
	}
	return true, nil
}

func updateRechargeOrderStatusTx(ctx context.Context, tx pgx.Tx, order *RechargeOrder, status string) error {
	if order == nil {
		return fmt.Errorf("update recharge order status: empty order")
	}
	if order.Status == rechargeStatusPaid {
		return nil
	}
	if status == "" || status == order.Status {
		return nil
	}

	const query = `
		UPDATE recharge_orders
		SET status = $2, updated_at = now()
		WHERE id = $1
		RETURNING updated_at
	`
	if err := tx.QueryRow(ctx, query, order.ID, status).Scan(&order.UpdatedAt); err != nil {
		return fmt.Errorf("update recharge order status: %w", err)
	}
	order.Status = status
	return nil
}

func loadRechargeSpiritStonesForUpdateTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID) (int64, error) {
	const query = `
		SELECT spirit_stones
		FROM player_resources
		WHERE user_id = $1
		FOR UPDATE
	`
	var value int64
	if err := tx.QueryRow(ctx, query, userID).Scan(&value); err != nil {
		return 0, fmt.Errorf("load spirit stones for recharge: %w", err)
	}
	return value, nil
}

func updateRechargeSpiritStonesTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID, value int64) error {
	const query = `
		UPDATE player_resources
		SET spirit_stones = $2, updated_at = now()
		WHERE user_id = $1
	`
	if _, err := tx.Exec(ctx, query, userID, value); err != nil {
		return fmt.Errorf("update spirit stones for recharge: %w", err)
	}
	return nil
}

func insertRechargeEconomyLogTx(
	ctx context.Context,
	tx pgx.Tx,
	userID uuid.UUID,
	changeType string,
	amount int64,
	balanceAfter int64,
	detail string,
) error {
	const query = `
		INSERT INTO economy_logs (user_id, currency, change_type, amount, balance_after, detail, occurred_at)
		VALUES ($1, 'spirit_stones', $2, $3, $4, $5, now())
	`
	if _, err := tx.Exec(ctx, query, userID, changeType, amount, balanceAfter, detail); err != nil {
		return fmt.Errorf("insert recharge economy log: %w", err)
	}
	return nil
}

type rechargeOrderScanner interface {
	Scan(dest ...any) error
}

func scanRechargeOrder(scanner rechargeOrderScanner) (RechargeOrder, error) {
	order := RechargeOrder{}
	var (
		paidAt sql.NullTime
	)
	if err := scanner.Scan(
		&order.ID,
		&order.UserID,
		&order.ProductCode,
		&order.CreditAmount,
		&order.SpiritStones,
		&order.ExternalOrderID,
		&order.Status,
		&order.IdempotencyKey,
		&order.CreatedAt,
		&paidAt,
		&order.UpdatedAt,
	); err != nil {
		return RechargeOrder{}, fmt.Errorf("scan recharge order row: %w", err)
	}
	if paidAt.Valid {
		ts := paidAt.Time.UTC()
		order.PaidAt = &ts
	}
	return order, nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
