package service

import (
	"sort"
	"sync"

	"github.com/google/uuid"
)

const (
	GameRealtimeTopicSnapshot   = "snapshot"
	GameRealtimeTopicMeditation = "meditation"
	GameRealtimeTopicHunting    = "hunting"
	GameRealtimeTopicAll        = "all"
)

type GameRealtimeNotification struct {
	Topics []string
}

type GameRealtimeBroker struct {
	mu          sync.RWMutex
	subscribers map[uuid.UUID]map[chan GameRealtimeNotification]struct{}
}

func NewGameRealtimeBroker() *GameRealtimeBroker {
	return &GameRealtimeBroker{
		subscribers: make(map[uuid.UUID]map[chan GameRealtimeNotification]struct{}),
	}
}

func (b *GameRealtimeBroker) Subscribe(userID uuid.UUID) (<-chan GameRealtimeNotification, func()) {
	ch := make(chan GameRealtimeNotification, 1)

	b.mu.Lock()
	if _, ok := b.subscribers[userID]; !ok {
		b.subscribers[userID] = make(map[chan GameRealtimeNotification]struct{})
	}
	b.subscribers[userID][ch] = struct{}{}
	b.mu.Unlock()

	unsubscribe := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		userSubs, ok := b.subscribers[userID]
		if !ok {
			return
		}
		if _, exists := userSubs[ch]; !exists {
			return
		}
		delete(userSubs, ch)
		close(ch)
		if len(userSubs) == 0 {
			delete(b.subscribers, userID)
		}
	}

	return ch, unsubscribe
}

func (b *GameRealtimeBroker) Publish(userID uuid.UUID, topic string) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	normalizedTopic := normalizeGameRealtimeTopic(topic)
	for ch := range b.subscribers[userID] {
		mergedTopics := map[string]struct{}{normalizedTopic: {}}
		select {
		case queued := <-ch:
			for _, queuedTopic := range queued.Topics {
				mergedTopics[normalizeGameRealtimeTopic(queuedTopic)] = struct{}{}
			}
		default:
		}

		select {
		case ch <- GameRealtimeNotification{Topics: compactRealtimeTopics(mergedTopics)}:
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
	case GameRealtimeTopicSnapshot:
		return GameRealtimeTopicSnapshot
	default:
		return GameRealtimeTopicAll
	}
}
