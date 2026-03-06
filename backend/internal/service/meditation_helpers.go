package service

import (
	"encoding/json"
	"math"
	"time"
)

const (
	defaultMeditationOfflineCap = 12 * time.Hour
	defaultMeditationCapSeconds = 12 * 60 * 60
)

type meditationEffectBonus struct {
	SpiritRateBonus float64
	SpiritCapBonus  float64
}

func baseMeditationSpiritRegen(level int) float64 {
	if level <= 0 {
		level = 1
	}
	realmIndex := (level - 1) / 9
	minorIndex := (level - 1) % 9

	baseRate := 10 * math.Pow(1.75, float64(realmIndex)) * (1 + float64(minorIndex)*0.05)
	if baseRate < 1 {
		return 1
	}
	return baseRate
}

func resolveMeditationSpiritRegen(level int, spiritRate float64, bonus meditationEffectBonus) float64 {
	if spiritRate <= 0 {
		spiritRate = 1
	}
	totalMultiplier := spiritRate * (1 + bonus.SpiritRateBonus)
	if totalMultiplier < 0.1 {
		totalMultiplier = 0.1
	}
	return math.Max(1, baseMeditationSpiritRegen(level)*totalMultiplier)
}

func resolveMeditationSpiritCap(level int, bonus meditationEffectBonus) float64 {
	capMultiplier := 1 + bonus.SpiritCapBonus
	if capMultiplier < 0.2 {
		capMultiplier = 0.2
	}

	capValue := baseMeditationSpiritRegen(level) * float64(defaultMeditationCapSeconds) * capMultiplier
	if capValue < 100 {
		return 100
	}
	return capValue
}

func clampSpiritValueToCap(spirit float64, spiritCap float64) float64 {
	if spiritCap <= 0 {
		return math.Max(0, spirit)
	}
	if spirit < 0 {
		return 0
	}
	if spirit > spiritCap {
		return spiritCap
	}
	return spirit
}

func recoverableSpiritCap(currentSpirit float64, spiritCap float64) float64 {
	if currentSpirit > spiritCap {
		return currentSpirit
	}
	return spiritCap
}

func resolveMeditationEffectBonus(effects []map[string]any, nowMilli int64) ([]map[string]any, meditationEffectBonus) {
	filtered := make([]map[string]any, 0, len(effects))
	bonus := meditationEffectBonus{}

	for _, effect := range effects {
		if inventoryReadInt64(effect["endTime"], 0) <= nowMilli {
			continue
		}
		filtered = append(filtered, effect)

		effectType := inventoryReadString(effect["type"])
		effectValue := inventoryReadFloat(effect["value"], 0)
		switch effectType {
		case "spiritRate":
			bonus.SpiritRateBonus += effectValue
		case "spiritCap":
			bonus.SpiritCapBonus += effectValue
		}
	}

	if bonus.SpiritRateBonus > 5 {
		bonus.SpiritRateBonus = 5
	}
	if bonus.SpiritCapBonus > 10 {
		bonus.SpiritCapBonus = 10
	}
	return filtered, bonus
}

func decodeMeditationActiveEffects(raw []byte, nowMilli int64) ([]map[string]any, meditationEffectBonus) {
	effects := make([]map[string]any, 0)
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &effects)
	}
	if effects == nil {
		effects = []map[string]any{}
	}
	return resolveMeditationEffectBonus(effects, nowMilli)
}

func calculateSpiritRecoveryAmount(
	level int,
	spiritRate float64,
	activeEffects []map[string]any,
	effect map[string]any,
	nowMilli int64,
) float64 {
	_, bonus := resolveMeditationEffectBonus(activeEffects, nowMilli)

	durationSeconds := float64(inventoryReadInt(effect["duration"], 0))
	if durationSeconds <= 0 {
		durationSeconds = 600
	}
	recoveryFactor := inventoryReadFloat(effect["value"], 0)
	if recoveryFactor <= 0 {
		recoveryFactor = 0.25
	}

	recovery := resolveMeditationSpiritRegen(level, spiritRate, bonus) * durationSeconds * recoveryFactor
	if recovery < 1 {
		return 1
	}
	return recovery
}
