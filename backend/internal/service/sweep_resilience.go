package service

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

const sweepPerUserAdvanceTimeout = 2 * time.Second
const sweepFailureStopThreshold = 3

func isSweepSkippableError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "40P01", // deadlock_detected
			"55P03", // lock_not_available
			"57014": // query_canceled
			return true
		}
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "deadlock detected") ||
		strings.Contains(msg, "lock timeout") ||
		strings.Contains(msg, "statement timeout")
}

func wrapSweepUserAdvance(fn func() error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic during sweep advance: %v\n%s", recovered, strings.TrimSpace(string(debug.Stack())))
		}
	}()
	return fn()
}

func trimSweepError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if len(message) <= 500 {
		return message
	}
	return message[:500]
}
