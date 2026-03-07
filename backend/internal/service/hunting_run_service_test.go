package service

import (
	"encoding/json"
	"math/rand"
	"testing"
	"time"
)

func TestResolveHuntingEncounterExhausted(t *testing.T) {
	targetMap, ok := findHuntingMapByID("qingmu_forest")
	if !ok {
		t.Fatal("expected hunting map")
	}

	state := testHuntingRunState(map[string]float64{"health": 120, "attack": 20, "defense": 10, "speed": 10})
	state.Spirit = 0
	now := time.Unix(1700000000, 0)

	outcome, nextItems, nextHerbs, err := resolveHuntingEncounter(state, targetMap, nil, nil, huntingEncounterConfig{
		OccurredAt:       now,
		GainMultiplier:   2,
		ReviveMultiplier: 1,
		HealBaseRate:     0,
		HealCapRate:      0,
		RefundChance:     0,
		RefundMinRatio:   0,
		RefundMaxRatio:   0,
		RNG:              rand.New(rand.NewSource(1)),
	})
	if err != nil {
		t.Fatalf("resolve encounter: %v", err)
	}
	if outcome.State != huntingRunStateExhausted {
		t.Fatalf("expected exhausted, got %s", outcome.State)
	}
	if state.RunActive {
		t.Fatal("expected run inactive after exhaustion")
	}
	if state.TotalSpiritCost != 0 {
		t.Fatalf("expected no spirit cost, got %d", state.TotalSpiritCost)
	}
	if len(nextItems) != 0 || len(nextHerbs) != 0 {
		t.Fatal("expected no inventory changes on exhaustion")
	}
	if outcome.LogMessage == "" {
		t.Fatal("expected exhaustion log message")
	}
}

func TestResolveHuntingEncounterVictory(t *testing.T) {
	targetMap, ok := findHuntingMapByID("qingmu_forest")
	if !ok {
		t.Fatal("expected hunting map")
	}

	state := testHuntingRunState(map[string]float64{"health": 2000, "attack": 500, "defense": 150, "speed": 40})
	state.Spirit = 1000
	state.CultivationRate = 1
	now := time.Unix(1700000100, 0)

	outcome, _, _, err := resolveHuntingEncounter(state, targetMap, nil, nil, huntingEncounterConfig{
		OccurredAt:       now,
		GainMultiplier:   2,
		ReviveMultiplier: 1,
		HealBaseRate:     0.1,
		HealCapRate:      0.45,
		RefundChance:     0,
		RefundMinRatio:   0,
		RefundMaxRatio:   0,
		RNG:              rand.New(rand.NewSource(2)),
	})
	if err != nil {
		t.Fatalf("resolve encounter: %v", err)
	}
	if outcome.State != huntingRunStateRunning {
		t.Fatalf("expected running, got %s", outcome.State)
	}
	if outcome.CultivationGain <= 0 {
		t.Fatalf("expected cultivation gain, got %d", outcome.CultivationGain)
	}
	if state.KillCount != 1 {
		t.Fatalf("expected kill count 1, got %d", state.KillCount)
	}
	if state.TotalCultivationGain != outcome.CultivationGain {
		t.Fatalf("expected total gain %d, got %d", outcome.CultivationGain, state.TotalCultivationGain)
	}
	if state.CurrentHP <= 0 {
		t.Fatalf("expected positive hp after victory, got %.2f", state.CurrentHP)
	}
	if !state.ReviveUntil.IsZero() {
		t.Fatal("expected no revive timer after victory")
	}
}

func TestResolveHuntingEncounterDefeatStartsRevive(t *testing.T) {
	targetMap, ok := findHuntingMapByID("qingmu_forest")
	if !ok {
		t.Fatal("expected hunting map")
	}

	state := testHuntingRunState(map[string]float64{"health": 30, "attack": 2, "defense": 0, "speed": 1})
	state.Spirit = 100
	now := time.Unix(1700000200, 0)

	outcome, _, _, err := resolveHuntingEncounter(state, targetMap, nil, nil, huntingEncounterConfig{
		OccurredAt:       now,
		GainMultiplier:   2,
		ReviveMultiplier: 1,
		HealBaseRate:     0,
		HealCapRate:      0,
		RefundChance:     0,
		RefundMinRatio:   0,
		RefundMaxRatio:   0,
		RNG:              rand.New(rand.NewSource(3)),
	})
	if err != nil {
		t.Fatalf("resolve encounter: %v", err)
	}
	if outcome.State != huntingRunStateReviving {
		t.Fatalf("expected reviving, got %s", outcome.State)
	}
	if state.CurrentHP != 0 {
		t.Fatalf("expected hp 0 after defeat, got %.2f", state.CurrentHP)
	}
	if !state.ReviveUntil.After(now) {
		t.Fatal("expected revive timer after defeat")
	}
	if state.TotalSpiritCost <= 0 {
		t.Fatalf("expected spirit cost consumed, got %d", state.TotalSpiritCost)
	}
	if outcome.LogMessage == "" {
		t.Fatal("expected defeat log message")
	}
}

func testHuntingRunState(baseAttrs map[string]float64) *huntingRunState {
	return &huntingRunState{
		RunActive:           true,
		Level:               1,
		Realm:               "练气",
		Cultivation:         0,
		MaxCultivation:      100,
		SpiritRate:          1,
		Luck:                0,
		CultivationRate:     1,
		CurrentHP:           baseAttrs["health"],
		MaxHP:               baseAttrs["health"],
		BaseAttributesRaw:   mustMarshalFloatMap(baseAttrs),
		CombatAttributesRaw: mustMarshalFloatMap(map[string]float64{}),
		CombatResistRaw:     mustMarshalFloatMap(map[string]float64{}),
		SpecialAttrsRaw:     mustMarshalFloatMap(map[string]float64{}),
	}
}

func mustMarshalFloatMap(values map[string]float64) []byte {
	raw, err := json.Marshal(values)
	if err != nil {
		panic(err)
	}
	return raw
}
