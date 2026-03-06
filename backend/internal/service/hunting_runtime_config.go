package service

import (
	"math"
	"math/rand"
	"time"
)

const (
	defaultHuntingReviveMultiplier     = 1.0
	defaultHuntingAutoHealBaseRate     = 0.10
	defaultHuntingAutoHealCapRate      = 0.45
	defaultHuntingSpiritRefundChance   = 0.20
	defaultHuntingSpiritRefundMinRatio = 0.10
	defaultHuntingSpiritRefundMaxRatio = 0.35
	defaultHuntingOfflineCap           = 12 * time.Hour
)

func resolveHuntingReviveDuration(base time.Duration, multiplier float64) time.Duration {
	if base <= 0 {
		base = 5 * time.Second
	}
	if multiplier < 0.2 {
		multiplier = 0.2
	}
	if multiplier > 5 {
		multiplier = 5
	}

	scaled := time.Duration(float64(base) * multiplier)
	if scaled < time.Second {
		return time.Second
	}
	if scaled > 30*time.Minute {
		return 30 * time.Minute
	}
	return scaled
}

func resolveHuntingHealRate(base float64, cap float64, bonus float64) float64 {
	if base < 0 {
		base = 0
	}
	if base > 1 {
		base = 1
	}
	if cap < 0 {
		cap = 0
	}
	if cap > 1 {
		cap = 1
	}
	if cap < base {
		cap = base
	}

	total := base + bonus
	if total < 0 {
		total = 0
	}
	if total > cap {
		total = cap
	}
	return total
}

func resolveHuntingSpiritRefundConfig(chance float64, minRatio float64, maxRatio float64) (float64, float64, float64) {
	if chance < 0 {
		chance = 0
	}
	if chance > 1 {
		chance = 1
	}
	if minRatio < 0 {
		minRatio = 0
	}
	if minRatio > 1 {
		minRatio = 1
	}
	if maxRatio < 0 {
		maxRatio = 0
	}
	if maxRatio > 1 {
		maxRatio = 1
	}
	if maxRatio < minRatio {
		maxRatio = minRatio
	}
	return chance, minRatio, maxRatio
}

func rollHuntingSpiritRefund(
	spiritCost int64,
	chance float64,
	minRatio float64,
	maxRatio float64,
	rng *rand.Rand,
) int64 {
	if spiritCost <= 0 || rng == nil {
		return 0
	}
	chance, minRatio, maxRatio = resolveHuntingSpiritRefundConfig(chance, minRatio, maxRatio)
	if chance <= 0 || maxRatio <= 0 {
		return 0
	}
	if rng.Float64() > chance {
		return 0
	}

	ratio := minRatio
	if maxRatio > minRatio {
		ratio = minRatio + rng.Float64()*(maxRatio-minRatio)
	}

	refund := int64(math.Round(float64(spiritCost) * ratio))
	if refund < 1 {
		refund = 1
	}
	if refund > spiritCost {
		refund = spiritCost
	}
	return refund
}
