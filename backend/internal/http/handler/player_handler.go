package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kowming/vue-idle-xiuxian/backend/internal/http/middleware"
	"github.com/kowming/vue-idle-xiuxian/backend/internal/repository"
)

type PlayerHandler struct {
	userRepo *repository.UserRepository
}

func NewPlayerHandler(userRepo *repository.UserRepository) *PlayerHandler {
	return &PlayerHandler{userRepo: userRepo}
}

func (h *PlayerHandler) Snapshot(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	snapshot, err := h.userRepo.GetSnapshot(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query snapshot failed"})
		return
	}
	if snapshot == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "snapshot not found"})
		return
	}

	c.JSON(http.StatusOK, buildPlayerSnapshotPayload(snapshot))
}

func buildPlayerSnapshotPayload(snapshot *repository.PlayerSnapshot) gin.H {
	if snapshot == nil {
		return gin.H{}
	}

	return gin.H{
		"name":                    snapshot.Name,
		"level":                   snapshot.Level,
		"realm":                   snapshot.Realm,
		"cultivation":             snapshot.Cultivation,
		"maxCultivation":          snapshot.MaxCultivation,
		"spirit":                  snapshot.Spirit,
		"spiritRate":              snapshot.SpiritRate,
		"luck":                    snapshot.Luck,
		"cultivationRate":         snapshot.CultivationRate,
		"spiritStones":            snapshot.SpiritStones,
		"reinforceStones":         snapshot.ReinforceStones,
		"refinementStones":        snapshot.RefinementStones,
		"petEssence":              snapshot.PetEssence,
		"explorationCount":        snapshot.ExplorationCount,
		"eventTriggered":          snapshot.EventTriggered,
		"dungeonHighestFloor":     snapshot.DungeonHighestFloor,
		"dungeonHighestFloor_2":   snapshot.DungeonHighestFloor2,
		"dungeonHighestFloor_5":   snapshot.DungeonHighestFloor5,
		"dungeonHighestFloor_10":  snapshot.DungeonHighestFloor10,
		"dungeonHighestFloor_100": snapshot.DungeonHighestFloor100,
		"dungeonLastFailedFloor":  snapshot.DungeonLastFailedFloor,
		"dungeonTotalRuns":        snapshot.DungeonTotalRuns,
		"dungeonBossKills":        snapshot.DungeonBossKills,
		"dungeonEliteKills":       snapshot.DungeonEliteKills,
		"dungeonTotalKills":       snapshot.DungeonTotalKills,
		"dungeonDeathCount":       snapshot.DungeonDeathCount,
		"dungeonTotalRewards":     snapshot.DungeonTotalRewards,
		"baseAttributes":          decodeJSON(snapshot.BaseAttributes),
		"combatAttributes":        decodeJSON(snapshot.CombatAttributes),
		"combatResistance":        decodeJSON(snapshot.CombatResistance),
		"specialAttributes":       decodeJSON(snapshot.SpecialAttributes),
		"herbs":                   decodeJSONArray(snapshot.Herbs),
		"pillFragments":           decodeJSON(snapshot.PillFragments),
		"pillRecipes":             decodeStringArray(snapshot.PillRecipes),
		"items":                   decodeJSONArray(snapshot.Items),
		"activePetId":             snapshot.ActivePetID,
		"activeEffects":           decodeJSONArray(snapshot.ActiveEffects),
		"equippedArtifacts":       decodeJSON(snapshot.EquippedArtifacts),
	}
}

func (h *PlayerHandler) ActiveCount(c *gin.Context) {
	_, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	const activeWithin = 12 * time.Hour
	count, err := h.userRepo.CountActivePlayers(c.Request.Context(), activeWithin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query active players failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"activeUsers": count,
		"windowHours": int(activeWithin / time.Hour),
	})
}

func decodeJSON(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	decoded := make(map[string]any)
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return map[string]any{}
	}
	return decoded
}

func decodeJSONArray(raw []byte) []any {
	if len(raw) == 0 {
		return []any{}
	}
	decoded := make([]any, 0)
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return []any{}
	}
	return decoded
}

func decodeStringArray(raw []byte) []string {
	if len(raw) == 0 {
		return []string{}
	}
	decoded := make([]string, 0)
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return []string{}
	}
	return decoded
}
