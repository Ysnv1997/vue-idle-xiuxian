package service

import (
	"log"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
)

const (
	GameRealtimeTopicSnapshot    = "snapshot"
	GameRealtimeTopicMeditation  = "meditation"
	GameRealtimeTopicHunting     = "hunting"
	GameRealtimeTopicExploration = "exploration"
	GameRealtimeTopicAll         = "all"
)

type GameRealtimeNotification struct {
	Topics []string
}

type GameRealtimeBroker struct {
	mu                      sync.RWMutex
	subscribers             map[uuid.UUID]map[chan GameRealtimeNotification]struct{}
	announcementSubscribers map[chan WorldAnnouncement]struct{}
}

func NewGameRealtimeBroker() *GameRealtimeBroker {
	return &GameRealtimeBroker{
		subscribers:             make(map[uuid.UUID]map[chan GameRealtimeNotification]struct{}),
		announcementSubscribers: make(map[chan WorldAnnouncement]struct{}),
	}
}

func (b *GameRealtimeBroker) Subscribe(userID uuid.UUID) (<-chan GameRealtimeNotification, func()) {
	ch := make(chan GameRealtimeNotification, 1)

	b.mu.Lock()
	if _, ok := b.subscribers[userID]; !ok {
		b.subscribers[userID] = make(map[chan GameRealtimeNotification]struct{})
	}
	b.subscribers[userID][ch] = struct{}{}
	userSubsCount := len(b.subscribers[userID])
	totalUsers := len(b.subscribers)
	b.mu.Unlock()
	log.Printf(
		"realtime broker subscribe user=%s user_subscribers=%d active_users=%d",
		userID.String(),
		userSubsCount,
		totalUsers,
	)

	unsubscribe := func() {
		b.mu.Lock()
		userSubs, ok := b.subscribers[userID]
		if !ok {
			b.mu.Unlock()
			return
		}
		if _, exists := userSubs[ch]; !exists {
			b.mu.Unlock()
			return
		}
		delete(userSubs, ch)
		close(ch)
		userSubsCount := len(userSubs)
		if len(userSubs) == 0 {
			delete(b.subscribers, userID)
			userSubsCount = 0
		}
		totalUsers := len(b.subscribers)
		b.mu.Unlock()
		log.Printf(
			"realtime broker unsubscribe user=%s user_subscribers=%d active_users=%d",
			userID.String(),
			userSubsCount,
			totalUsers,
		)
	}

	return ch, unsubscribe
}

func (b *GameRealtimeBroker) SubscribeAnnouncements() (<-chan WorldAnnouncement, func()) {
	ch := make(chan WorldAnnouncement, 8)
	b.mu.Lock()
	b.announcementSubscribers[ch] = struct{}{}
	subscribers := len(b.announcementSubscribers)
	b.mu.Unlock()
	log.Printf("realtime broker subscribe announcements subscribers=%d", subscribers)

	unsubscribe := func() {
		b.mu.Lock()
		if _, ok := b.announcementSubscribers[ch]; !ok {
			b.mu.Unlock()
			return
		}
		delete(b.announcementSubscribers, ch)
		close(ch)
		subscribers := len(b.announcementSubscribers)
		b.mu.Unlock()
		log.Printf("realtime broker unsubscribe announcements subscribers=%d", subscribers)
	}

	return ch, unsubscribe
}

func (b *GameRealtimeBroker) Publish(userID uuid.UUID, topic string) {
	b.mu.RLock()
	userSubs, hasSubs := b.subscribers[userID]
	if !hasSubs || len(userSubs) == 0 {
		b.mu.RUnlock()
		log.Printf(
			"realtime broker publish user=%s topic=%s subscribers=0 skipped=true",
			userID.String(),
			normalizeGameRealtimeTopic(topic),
		)
		return
	}
	normalizedTopic := normalizeGameRealtimeTopic(topic)
	delivered := 0
	dropped := 0
	mergedQueuedTopics := 0
	for ch := range userSubs {
		mergedTopics := map[string]struct{}{normalizedTopic: {}}
		select {
		case queued := <-ch:
			for _, queuedTopic := range queued.Topics {
				mergedTopics[normalizeGameRealtimeTopic(queuedTopic)] = struct{}{}
			}
			mergedQueuedTopics += len(queued.Topics)
		default:
		}

		notification := GameRealtimeNotification{Topics: compactRealtimeTopics(mergedTopics)}
		select {
		case ch <- notification:
			delivered++
		default:
			dropped++
		}
	}
	subscribers := len(userSubs)
	b.mu.RUnlock()
	log.Printf(
		"realtime broker publish user=%s topic=%s subscribers=%d delivered=%d dropped=%d merged_queued_topics=%d",
		userID.String(),
		normalizedTopic,
		subscribers,
		delivered,
		dropped,
		mergedQueuedTopics,
	)
}

func (b *GameRealtimeBroker) PublishAnnouncement(announcement WorldAnnouncement) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if len(b.announcementSubscribers) == 0 {
		return
	}
	for ch := range b.announcementSubscribers {
		select {
		case ch <- announcement:
		default:
		}
	}
}

func compactRealtimeTopics(topics map[string]struct{}) []string {
	if len(topics) == 0 {
		return []string{GameRealtimeTopicAll}
	}
	if _, hasAll := topics[GameRealtimeTopicAll]; hasAll {
		return []string{GameRealtimeTopicAll}
	}
	result := make([]string, 0, len(topics))
	for topic := range topics {
		result = append(result, normalizeGameRealtimeTopic(topic))
	}
	sort.Strings(result)
	return result
}

func normalizeGameRealtimeTopic(topic string) string {
	switch topic {
	case GameRealtimeTopicMeditation:
		return GameRealtimeTopicMeditation
	case GameRealtimeTopicHunting:
		return GameRealtimeTopicHunting
	case GameRealtimeTopicExploration:
		return GameRealtimeTopicExploration
	case GameRealtimeTopicSnapshot:
		return GameRealtimeTopicSnapshot
	default:
		return GameRealtimeTopicAll
	}
}

func realtimeTopicsForLog(topics []string) string {
	if len(topics) == 0 {
		return "-"
	}
	normalized := make([]string, 0, len(topics))
	for _, topic := range topics {
		normalized = append(normalized, normalizeGameRealtimeTopic(topic))
	}
	sort.Strings(normalized)
	return strings.Join(normalized, ",")
}
