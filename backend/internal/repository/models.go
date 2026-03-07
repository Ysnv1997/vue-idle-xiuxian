package repository

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID              uuid.UUID
	LinuxDoUserID   string
	LinuxDoUsername string
	LinuxDoAvatar   string
	LastLoginAt     time.Time
}

type PlayerSnapshot struct {
	UserID uuid.UUID `json:"userId"`

	Name            string  `json:"name"`
	Level           int     `json:"level"`
	Realm           string  `json:"realm"`
	Cultivation     int64   `json:"cultivation"`
	MaxCultivation  int64   `json:"maxCultivation"`
	Spirit          float64 `json:"spirit"`
	SpiritRate      float64 `json:"spiritRate"`
	Luck            float64 `json:"luck"`
	CultivationRate float64 `json:"cultivationRate"`

	SpiritStones           int64 `json:"spiritStones"`
	ReinforceStones        int64 `json:"reinforceStones"`
	RefinementStones       int64 `json:"refinementStones"`
	PetEssence             int64 `json:"petEssence"`
	ExplorationCount       int64 `json:"explorationCount"`
	EventTriggered         int64 `json:"eventTriggered"`
	DungeonHighestFloor    int64 `json:"dungeonHighestFloor"`
	DungeonHighestFloor2   int64 `json:"dungeonHighestFloor_2"`
	DungeonHighestFloor5   int64 `json:"dungeonHighestFloor_5"`
	DungeonHighestFloor10  int64 `json:"dungeonHighestFloor_10"`
	DungeonHighestFloor100 int64 `json:"dungeonHighestFloor_100"`
	DungeonLastFailedFloor int64 `json:"dungeonLastFailedFloor"`
	DungeonTotalRuns       int64 `json:"dungeonTotalRuns"`
	DungeonBossKills       int64 `json:"dungeonBossKills"`
	DungeonEliteKills      int64 `json:"dungeonEliteKills"`
	DungeonTotalKills      int64 `json:"dungeonTotalKills"`
	DungeonDeathCount      int64 `json:"dungeonDeathCount"`
	DungeonTotalRewards    int64 `json:"dungeonTotalRewards"`

	BaseAttributes    json.RawMessage `json:"baseAttributes"`
	CombatAttributes  json.RawMessage `json:"combatAttributes"`
	CombatResistance  json.RawMessage `json:"combatResistance"`
	SpecialAttributes json.RawMessage `json:"specialAttributes"`
	Herbs             json.RawMessage `json:"herbs"`
	PillFragments     json.RawMessage `json:"pillFragments"`
	PillRecipes       json.RawMessage `json:"pillRecipes"`
	Items             json.RawMessage `json:"items"`
	EquippedArtifacts json.RawMessage `json:"equippedArtifacts"`
	ActivePetID       string          `json:"activePetId"`
	ActiveEffects     json.RawMessage `json:"activeEffects"`
}

type PublicPlayerProfile struct {
	UserID            uuid.UUID       `json:"userId"`
	Name              string          `json:"name"`
	Level             int             `json:"level"`
	Realm             string          `json:"realm"`
	BaseAttributes    json.RawMessage `json:"baseAttributes"`
	CombatAttributes  json.RawMessage `json:"combatAttributes"`
	CombatResistance  json.RawMessage `json:"combatResistance"`
	SpecialAttributes json.RawMessage `json:"specialAttributes"`
	EquippedArtifacts json.RawMessage `json:"equippedArtifacts"`
	ActivePetID       string          `json:"activePetId"`
	Items             json.RawMessage `json:"items"`
}
