package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/kowming/vue-idle-xiuxian/backend/internal/repository"
	"github.com/kowming/vue-idle-xiuxian/backend/internal/service"
)

const (
	gameRealtimeIdleKeepaliveInterval = 30 * time.Second
)

type GameRealtimeHandler struct {
	tokenService           *service.TokenService
	passiveProgressService *service.PassiveProgressService
	gameService            *service.GameService
	userRepo               *repository.UserRepository
	realtimeBroker         *service.GameRealtimeBroker
	upgrader               websocket.Upgrader
}

func NewGameRealtimeHandler(
	tokenService *service.TokenService,
	passiveProgressService *service.PassiveProgressService,
	gameService *service.GameService,
	userRepo *repository.UserRepository,
	realtimeBroker *service.GameRealtimeBroker,
) *GameRealtimeHandler {
	return &GameRealtimeHandler{
		tokenService:           tokenService,
		passiveProgressService: passiveProgressService,
		gameService:            gameService,
		userRepo:               userRepo,
		realtimeBroker:         realtimeBroker,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(_ *http.Request) bool {
				return true
			},
		},
	}
}

func (h *GameRealtimeHandler) Connect(c *gin.Context) {
	accessToken := strings.TrimSpace(c.Query("accessToken"))
	if accessToken == "" {
		header := strings.TrimSpace(c.GetHeader("Authorization"))
		parts := strings.SplitN(header, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			accessToken = strings.TrimSpace(parts[1])
		}
	}
	if accessToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing access token"})
		return
	}

	claims, err := h.tokenService.ValidateToken(accessToken, "access")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid access token"})
		return
	}
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id in token"})
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer func() {
		_ = conn.Close()
	}()

	var writeMu sync.Mutex
	writeEnvelope := func(event string, data any) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		return conn.WriteJSON(gin.H{
			"event": event,
			"data":  data,
		})
	}

	lastSnapshotPayload := []byte(nil)
	lastSnapshotState := map[string]any(nil)
	lastMeditationPayload := []byte(nil)
	lastHuntingPayload := []byte(nil)

	if err := writeEnvelope("game.connected", gin.H{"userId": userID.String()}); err != nil {
		return
	}

	sync := func(topics map[string]struct{}, force bool) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if h.passiveProgressService != nil {
			_ = h.passiveProgressService.TouchActivity(ctx, userID)
		}

		wantsAll := force || hasRealtimeTopic(topics, service.GameRealtimeTopicAll)
		wantsMeditation := wantsAll || hasRealtimeTopic(topics, service.GameRealtimeTopicMeditation)
		wantsHunting := wantsAll || hasRealtimeTopic(topics, service.GameRealtimeTopicHunting)
		wantsSnapshot := wantsAll || hasRealtimeTopic(topics, service.GameRealtimeTopicSnapshot)

		if wantsMeditation {
			meditationStatus, err := h.gameService.MeditationStatus(ctx, userID)
			if err != nil {
				return err
			}
			meditationRaw, _ := json.Marshal(meditationStatus)
			if force || !jsonPayloadEqual(lastMeditationPayload, meditationRaw) {
				lastMeditationPayload = meditationRaw
				if err := writeEnvelope("game.meditation", meditationStatus); err != nil {
					return err
				}
			}
		}

		if wantsHunting {
			huntingStatus, err := h.gameService.HuntingStatus(ctx, userID)
			if err != nil {
				return err
			}
			huntingRaw, _ := json.Marshal(huntingStatus)
			if force || !jsonPayloadEqual(lastHuntingPayload, huntingRaw) {
				lastHuntingPayload = huntingRaw
				if err := writeEnvelope("game.hunting", huntingStatus); err != nil {
					return err
				}
			}
		}

		if wantsSnapshot {
			snapshot, err := h.userRepo.GetSnapshot(ctx, userID)
			if err != nil {
				return err
			}
			snapshotPayload := buildPlayerSnapshotPayload(snapshot)
			snapshotRaw, _ := json.Marshal(snapshotPayload)
			if force || !jsonPayloadEqual(lastSnapshotPayload, snapshotRaw) {
				wasInitialSnapshot := lastSnapshotState == nil
				deltaPayload := computeSnapshotDelta(lastSnapshotState, snapshotPayload)
				lastSnapshotPayload = snapshotRaw
				lastSnapshotState = cloneMap(snapshotPayload)
				eventName := "player.delta"
				eventPayload := any(deltaPayload)
				if force || wasInitialSnapshot || len(deltaPayload) == 0 {
					eventName = "player.snapshot"
					eventPayload = snapshotPayload
				}
				if err := writeEnvelope(eventName, eventPayload); err != nil {
					return err
				}
			}
		}

		return nil
	}

	if err := sync(map[string]struct{}{service.GameRealtimeTopicAll: {}}, true); err != nil {
		_ = writeEnvelope("game.error", gin.H{"error": "initial realtime sync failed"})
		return
	}

	readDone := make(chan struct{})
	notifyCh := (<-chan service.GameRealtimeNotification)(nil)
	unsubscribe := func() {}
	if h.realtimeBroker != nil {
		notifyCh, unsubscribe = h.realtimeBroker.Subscribe(userID)
	}
	defer unsubscribe()

	go func() {
		defer close(readDone)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	ticker := time.NewTicker(gameRealtimeIdleKeepaliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-readDone:
			return
		case notification, ok := <-notifyCh:
			if !ok {
				return
			}
			topics := make(map[string]struct{}, len(notification.Topics))
			for _, topic := range notification.Topics {
				topics[topic] = struct{}{}
			}
			if err := sync(topics, false); err != nil {
				_ = writeEnvelope("game.error", gin.H{"error": "realtime sync failed"})
				return
			}
		case <-ticker.C:
			if err := sync(map[string]struct{}{service.GameRealtimeTopicAll: {}}, false); err != nil {
				_ = writeEnvelope("game.error", gin.H{"error": "realtime sync failed"})
				return
			}
		}
	}
}

func hasRealtimeTopic(topics map[string]struct{}, topic string) bool {
	_, ok := topics[topic]
	return ok
}

func jsonPayloadEqual(left []byte, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	if len(left) == 0 && len(right) == 0 {
		return true
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func computeSnapshotDelta(previous map[string]any, current map[string]any) map[string]any {
	if len(current) == 0 {
		return map[string]any{}
	}
	if len(previous) == 0 {
		return cloneMap(current)
	}

	delta := make(map[string]any)
	for key, currentValue := range current {
		previousValue, ok := previous[key]
		if !ok || !reflect.DeepEqual(previousValue, currentValue) {
			delta[key] = currentValue
		}
	}
	return delta
}

func cloneMap(source map[string]any) map[string]any {
	if len(source) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}
