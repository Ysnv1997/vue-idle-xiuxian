package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"reflect"
	"sort"
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
	gameRealtimeSyncTimeout           = 10 * time.Second
)

type GameRealtimeHandler struct {
	tokenService           *service.TokenService
	passiveProgressService *service.PassiveProgressService
	gameService            *service.GameService
	explorationService     *service.ExplorationService
	userRepo               *repository.UserRepository
	realtimeBroker         *service.GameRealtimeBroker
	upgrader               websocket.Upgrader
}

func NewGameRealtimeHandler(
	tokenService *service.TokenService,
	passiveProgressService *service.PassiveProgressService,
	gameService *service.GameService,
	explorationService *service.ExplorationService,
	userRepo *repository.UserRepository,
	realtimeBroker *service.GameRealtimeBroker,
) *GameRealtimeHandler {
	return &GameRealtimeHandler{
		tokenService:           tokenService,
		passiveProgressService: passiveProgressService,
		gameService:            gameService,
		explorationService:     explorationService,
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
		log.Printf("game realtime connect rejected: missing access token remote=%s", c.ClientIP())
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing access token"})
		return
	}

	claims, err := h.tokenService.ValidateToken(accessToken, "access")
	if err != nil {
		log.Printf("game realtime connect rejected: invalid access token remote=%s err=%v", c.ClientIP(), err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid access token"})
		return
	}
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		log.Printf("game realtime connect rejected: invalid user id in token remote=%s user_id=%q err=%v", c.ClientIP(), claims.UserID, err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id in token"})
		return
	}
	log.Printf("game realtime connect start user=%s remote=%s", userID.String(), c.ClientIP())

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("game realtime upgrade failed user=%s remote=%s err=%v", userID.String(), c.ClientIP(), err)
		return
	}
	disconnectReason := "handler_return"
	defer func() {
		log.Printf("game realtime disconnect user=%s reason=%s", userID.String(), disconnectReason)
		_ = conn.Close()
	}()
	log.Printf("game realtime upgraded user=%s remote=%s", userID.String(), c.ClientIP())

	var writeMu sync.Mutex
	writeEnvelope := func(event string, data any) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		err := conn.WriteJSON(gin.H{
			"event": event,
			"data":  data,
		})
		if err != nil {
			log.Printf("game realtime write failed user=%s event=%s err=%v", userID.String(), event, err)
		}
		return err
	}

	lastSnapshotPayload := []byte(nil)
	lastSnapshotState := map[string]any(nil)
	lastMeditationPayload := []byte(nil)
	lastHuntingPayload := []byte(nil)
	lastExplorationPayload := []byte(nil)

	if err := writeEnvelope("game.connected", gin.H{"userId": userID.String()}); err != nil {
		disconnectReason = "write_connected_failed"
		return
	}

	syncCount := 0
	sync := func(topics map[string]struct{}, force bool) error {
		syncCount++
		startedAt := time.Now()
		topicsLog := realtimeTopicMapForLog(topics)
		log.Printf(
			"game realtime sync #%d start user=%s force=%t topics=%s",
			syncCount,
			userID.String(),
			force,
			topicsLog,
		)
		ctx, cancel := context.WithTimeout(context.Background(), gameRealtimeSyncTimeout)
		defer cancel()
		sentEvents := make([]string, 0, 4)
		send := func(event string, payload any) error {
			if err := writeEnvelope(event, payload); err != nil {
				return err
			}
			sentEvents = append(sentEvents, event)
			return nil
		}

		if h.passiveProgressService != nil {
			_ = h.passiveProgressService.TouchActivity(ctx, userID)
		}

		wantsAll := force || hasRealtimeTopic(topics, service.GameRealtimeTopicAll)
		wantsMeditation := wantsAll || hasRealtimeTopic(topics, service.GameRealtimeTopicMeditation)
		wantsHunting := wantsAll || hasRealtimeTopic(topics, service.GameRealtimeTopicHunting)
		wantsExploration := wantsAll || hasRealtimeTopic(topics, service.GameRealtimeTopicExploration)
		wantsSnapshot := wantsAll || hasRealtimeTopic(topics, service.GameRealtimeTopicSnapshot)

		if wantsMeditation {
			meditationStatus, err := h.gameService.MeditationStatus(ctx, userID)
			if err != nil {
				return err
			}
			meditationRaw, _ := json.Marshal(meditationStatus)
			if force || !jsonPayloadEqual(lastMeditationPayload, meditationRaw) {
				lastMeditationPayload = meditationRaw
				if err := send("game.meditation", meditationStatus); err != nil {
					log.Printf(
						"game realtime sync #%d failed user=%s stage=meditation elapsed=%s err=%v",
						syncCount,
						userID.String(),
						time.Since(startedAt),
						err,
					)
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
				if err := send("game.hunting", huntingStatus); err != nil {
					log.Printf(
						"game realtime sync #%d failed user=%s stage=hunting elapsed=%s err=%v",
						syncCount,
						userID.String(),
						time.Since(startedAt),
						err,
					)
					return err
				}
			}
		}

		if wantsExploration && h.explorationService != nil {
			explorationStatus, err := h.explorationService.ExplorationStatus(ctx, userID)
			if err != nil {
				return err
			}
			explorationRaw, _ := json.Marshal(explorationStatus)
			if force || !jsonPayloadEqual(lastExplorationPayload, explorationRaw) {
				lastExplorationPayload = explorationRaw
				if err := send("game.exploration", explorationStatus); err != nil {
					log.Printf(
						"game realtime sync #%d failed user=%s stage=exploration elapsed=%s err=%v",
						syncCount,
						userID.String(),
						time.Since(startedAt),
						err,
					)
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
				if err := send(eventName, eventPayload); err != nil {
					log.Printf(
						"game realtime sync #%d failed user=%s stage=%s elapsed=%s err=%v",
						syncCount,
						userID.String(),
						eventName,
						time.Since(startedAt),
						err,
					)
					return err
				}
			}
		}

		log.Printf(
			"game realtime sync #%d done user=%s force=%t topics=%s elapsed=%s sent_count=%d sent=%s",
			syncCount,
			userID.String(),
			force,
			topicsLog,
			time.Since(startedAt),
			len(sentEvents),
			strings.Join(sentEvents, ","),
		)
		return nil
	}

	if err := sync(map[string]struct{}{service.GameRealtimeTopicAll: {}}, true); err != nil {
		log.Printf("game realtime initial sync failed user=%s err=%v", userID.String(), err)
		_ = writeEnvelope("game.error", gin.H{"error": "initial realtime sync failed"})
	}

	readDone := make(chan struct{})
	notifyCh := (<-chan service.GameRealtimeNotification)(nil)
	announcementCh := (<-chan service.WorldAnnouncement)(nil)
	unsubscribe := func() {}
	unsubscribeAnnouncements := func() {}
	if h.realtimeBroker != nil {
		notifyCh, unsubscribe = h.realtimeBroker.Subscribe(userID)
		announcementCh, unsubscribeAnnouncements = h.realtimeBroker.SubscribeAnnouncements()
		log.Printf("game realtime subscribed user=%s", userID.String())
	} else {
		log.Printf("game realtime subscribe skipped user=%s reason=no_broker", userID.String())
	}
	defer unsubscribe()
	defer unsubscribeAnnouncements()

	go func() {
		defer close(readDone)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				log.Printf("game realtime read loop closed user=%s err=%v", userID.String(), err)
				return
			}
		}
	}()

	ticker := time.NewTicker(gameRealtimeIdleKeepaliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-readDone:
			disconnectReason = "client_closed"
			return
		case notification, ok := <-notifyCh:
			if !ok {
				log.Printf("game realtime notify channel closed user=%s", userID.String())
				disconnectReason = "notify_channel_closed"
				return
			}
			topics := make(map[string]struct{}, len(notification.Topics))
			for _, topic := range notification.Topics {
				topics[topic] = struct{}{}
			}
			log.Printf("game realtime notify received user=%s topics=%s", userID.String(), strings.Join(notification.Topics, ","))
			if err := sync(topics, false); err != nil {
				log.Printf("game realtime notify sync failed user=%s err=%v", userID.String(), err)
				_ = writeEnvelope("game.error", gin.H{"error": "realtime sync failed"})
				continue
			}
		case announcement, ok := <-announcementCh:
			if !ok {
				log.Printf("game realtime announcement channel closed user=%s", userID.String())
				disconnectReason = "announcement_channel_closed"
				return
			}
			if err := writeEnvelope("world.announcement", announcement); err != nil {
				log.Printf("game realtime announcement write failed user=%s err=%v", userID.String(), err)
				continue
			}
		case <-ticker.C:
			log.Printf("game realtime keepalive tick user=%s", userID.String())
			if err := sync(map[string]struct{}{service.GameRealtimeTopicAll: {}}, false); err != nil {
				log.Printf("game realtime keepalive sync failed user=%s err=%v", userID.String(), err)
				_ = writeEnvelope("game.error", gin.H{"error": "realtime sync failed"})
				continue
			}
		}
	}
}

func hasRealtimeTopic(topics map[string]struct{}, topic string) bool {
	_, ok := topics[topic]
	return ok
}

func realtimeTopicMapForLog(topics map[string]struct{}) string {
	if len(topics) == 0 {
		return "-"
	}
	list := make([]string, 0, len(topics))
	for topic := range topics {
		list = append(list, topic)
	}
	sort.Strings(list)
	return strings.Join(list, ",")
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
