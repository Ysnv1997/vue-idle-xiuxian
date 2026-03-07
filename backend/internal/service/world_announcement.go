package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	WorldAnnouncementCategoryBreakthrough = "breakthrough"
	WorldAnnouncementCategoryLoot         = "loot"
	WorldAnnouncementCategoryEnhance      = "enhance"
)

type WorldAnnouncement struct {
	ID        string `json:"id"`
	Category  string `json:"category"`
	Message   string `json:"message"`
	CreatedAt int64  `json:"createdAt"`
}

type WorldAnnouncementService struct {
	runtimeConfig *RuntimeConfigService
	broker        *GameRealtimeBroker

	mu             sync.Mutex
	lastBroadcast  time.Time
	lastByCategory map[string]time.Time
}

func NewWorldAnnouncementService(runtimeConfig *RuntimeConfigService, broker *GameRealtimeBroker) *WorldAnnouncementService {
	return &WorldAnnouncementService{
		runtimeConfig:  runtimeConfig,
		broker:         broker,
		lastByCategory: make(map[string]time.Time),
	}
}

func (s *WorldAnnouncementService) Publish(ctx context.Context, announcement WorldAnnouncement) {
	if s == nil || s.broker == nil {
		return
	}
	message := strings.TrimSpace(announcement.Message)
	if message == "" {
		return
	}
	if s.runtimeConfig != nil {
		if !s.runtimeConfig.GetBool(ctx, RuntimeConfigKeyWorldAnnouncementEnabled, true) {
			return
		}
		blockedKeywords := parseWorldAnnouncementBlockedKeywords(s.runtimeConfig.GetString(ctx, RuntimeConfigKeyWorldAnnouncementBlocked, ""))
		for _, keyword := range blockedKeywords {
			if strings.Contains(strings.ToLower(message), keyword) {
				return
			}
		}
		cooldownMS := s.runtimeConfig.GetInt(ctx, RuntimeConfigKeyWorldAnnouncementCooldownMS, 3000, 0, 60000)
		categoryCooldownMS := s.categoryCooldown(ctx, announcement.Category, cooldownMS)
		if cooldownMS > 0 {
			s.mu.Lock()
			now := time.Now()
			if !s.lastBroadcast.IsZero() && now.Sub(s.lastBroadcast) < time.Duration(cooldownMS)*time.Millisecond {
				s.mu.Unlock()
				return
			}
			if categoryCooldownMS > 0 {
				lastCategoryAt := s.lastByCategory[announcement.Category]
				if !lastCategoryAt.IsZero() && now.Sub(lastCategoryAt) < time.Duration(categoryCooldownMS)*time.Millisecond {
					s.mu.Unlock()
					return
				}
			}
			s.lastBroadcast = now
			s.lastByCategory[announcement.Category] = now
			s.mu.Unlock()
		}
	}
	log.Printf("world announcement category=%s message=%s", announcement.Category, message)
	s.broker.PublishAnnouncement(announcement)
}

func (s *WorldAnnouncementService) categoryCooldown(ctx context.Context, category string, fallback int) int {
	if s == nil || s.runtimeConfig == nil {
		return fallback
	}
	switch category {
	case WorldAnnouncementCategoryBreakthrough:
		return s.runtimeConfig.GetInt(ctx, RuntimeConfigKeyWorldAnnouncementCooldownBreakthroughMS, 10000, 0, 300000)
	case WorldAnnouncementCategoryLoot:
		return s.runtimeConfig.GetInt(ctx, RuntimeConfigKeyWorldAnnouncementCooldownLootMS, 3000, 0, 300000)
	case WorldAnnouncementCategoryEnhance:
		return s.runtimeConfig.GetInt(ctx, RuntimeConfigKeyWorldAnnouncementCooldownEnhanceMS, 5000, 0, 300000)
	default:
		return fallback
	}
}

func newWorldAnnouncement(category string, message string) WorldAnnouncement {
	return WorldAnnouncement{
		ID:        uuid.NewString(),
		Category:  category,
		Message:   strings.TrimSpace(message),
		CreatedAt: time.Now().UnixMilli(),
	}
}

func shouldAnnounceRareEquipmentQuality(quality string) bool {
	return quality == "legendary" || quality == "mythic"
}

func shouldAnnounceRarePetRarity(rarity string) bool {
	return rarity == "celestial" || rarity == "divine"
}

func majorRealmTransitionsBetween(previousLevel int, currentLevel int) []string {
	if currentLevel <= previousLevel {
		return nil
	}
	result := make([]string, 0, 2)
	for level := previousLevel + 1; level <= currentLevel; level++ {
		realm := realmByLevel(level).Name
		if shouldAnnounceMajorRealmBreakthrough(realm) {
			result = append(result, realm)
		}
	}
	return result
}

func shouldAnnounceMajorRealmBreakthrough(realmName string) bool {
	realmName = strings.TrimSpace(realmName)
	if !strings.HasSuffix(realmName, "一重") {
		return false
	}
	for _, prefix := range []string{"化神", "返虚", "合体", "大乘", "渡劫", "仙人", "真仙", "金仙", "太乙", "大罗"} {
		if strings.HasPrefix(realmName, prefix) {
			return true
		}
	}
	return false
}

func buildMajorRealmBreakthroughAnnouncement(playerName string, realmName string) WorldAnnouncement {
	return newWorldAnnouncement(
		WorldAnnouncementCategoryBreakthrough,
		fmt.Sprintf("天道震动！修士【%s】成功突破至【%s】，名震四方！", strings.TrimSpace(playerName), strings.TrimSpace(realmName)),
	)
}

func buildRareLootAnnouncement(playerName string, itemName string, rarityName string, itemKind string) WorldAnnouncement {
	return newWorldAnnouncement(
		WorldAnnouncementCategoryLoot,
		fmt.Sprintf("鸿运当头！修士【%s】获得%s%s【%s】！", strings.TrimSpace(playerName), strings.TrimSpace(rarityName), strings.TrimSpace(itemKind), strings.TrimSpace(itemName)),
	)
}

func buildEnhanceAnnouncement(playerName string, itemName string, level int) WorldAnnouncement {
	return newWorldAnnouncement(
		WorldAnnouncementCategoryEnhance,
		fmt.Sprintf("神兵现世！修士【%s】成功将【%s】强化至 +%d！", strings.TrimSpace(playerName), strings.TrimSpace(itemName), level),
	)
}

func parseWorldAnnouncementBlockedKeywords(raw string) []string {
	parts := strings.Split(strings.TrimSpace(raw), ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		keyword := strings.ToLower(strings.TrimSpace(part))
		if keyword == "" {
			continue
		}
		result = append(result, keyword)
	}
	return result
}

var defaultWorldAnnouncementService atomicWorldAnnouncementService

type atomicWorldAnnouncementService struct {
	mu      sync.RWMutex
	service *WorldAnnouncementService
}

func SetDefaultWorldAnnouncementService(service *WorldAnnouncementService) {
	defaultWorldAnnouncementService.mu.Lock()
	defaultWorldAnnouncementService.service = service
	defaultWorldAnnouncementService.mu.Unlock()
}

func publishWorldAnnouncement(ctx context.Context, broker *GameRealtimeBroker, announcement WorldAnnouncement) {
	defaultWorldAnnouncementService.mu.RLock()
	service := defaultWorldAnnouncementService.service
	defaultWorldAnnouncementService.mu.RUnlock()
	if service != nil {
		service.Publish(ctx, announcement)
		return
	}
	if broker == nil {
		return
	}
	broker.PublishAnnouncement(announcement)
}
