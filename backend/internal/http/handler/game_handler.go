package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/kowming/vue-idle-xiuxian/backend/internal/http/middleware"
	"github.com/kowming/vue-idle-xiuxian/backend/internal/service"
)

type GameHandler struct {
	gameService        *service.GameService
	explorationService *service.ExplorationService
	alchemyService     *service.AlchemyService
	gachaService       *service.GachaService
	inventoryService   *service.InventoryService
	equipmentService   *service.EquipmentService
	dungeonService     *service.DungeonService
	achievementService *service.AchievementService
}

func NewGameHandler(
	gameService *service.GameService,
	explorationService *service.ExplorationService,
	alchemyService *service.AlchemyService,
	gachaService *service.GachaService,
	inventoryService *service.InventoryService,
	equipmentService *service.EquipmentService,
	dungeonService *service.DungeonService,
	achievementService *service.AchievementService,
) *GameHandler {
	return &GameHandler{
		gameService:        gameService,
		explorationService: explorationService,
		alchemyService:     alchemyService,
		gachaService:       gachaService,
		inventoryService:   inventoryService,
		equipmentService:   equipmentService,
		dungeonService:     dungeonService,
		achievementService: achievementService,
	}
}

func (h *GameHandler) CultivationOnce(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	result, err := h.gameService.CultivateOnce(c.Request.Context(), userID)
	if err != nil {
		h.handleGameError(c, err)
		return
	}

	h.respondWithAchievementSync(c, userID, result)
}

func (h *GameHandler) CultivationUntilBreakthrough(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	result, err := h.gameService.CultivateUntilBreakthrough(c.Request.Context(), userID)
	if err != nil {
		h.handleGameError(c, err)
		return
	}

	h.respondWithAchievementSync(c, userID, result)
}

func (h *GameHandler) Breakthrough(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	result, err := h.gameService.Breakthrough(c.Request.Context(), userID)
	if err != nil {
		h.handleGameError(c, err)
		return
	}

	h.respondWithAchievementSync(c, userID, result)
}

type explorationStartRequest struct {
	LocationID string `json:"locationId"`
}

func (h *GameHandler) ExplorationStart(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req explorationStartRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.LocationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "locationId is required"})
		return
	}

	result, err := h.explorationService.Start(c.Request.Context(), userID, req.LocationID)
	if err != nil {
		h.handleGameError(c, err)
		return
	}

	h.respondWithAchievementSync(c, userID, result)
}

type alchemyCraftRequest struct {
	RecipeID string `json:"recipeId"`
}

func (h *GameHandler) AlchemyCraft(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req alchemyCraftRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.RecipeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "recipeId is required"})
		return
	}

	result, err := h.alchemyService.Craft(c.Request.Context(), userID, req.RecipeID)
	if err != nil {
		h.handleGameError(c, err)
		return
	}

	h.respondWithAchievementSync(c, userID, result)
}

type gachaDrawRequest struct {
	GachaType                string   `json:"gachaType"`
	Times                    int      `json:"times"`
	WishlistEnabled          bool     `json:"wishlistEnabled"`
	SelectedWishEquipQuality string   `json:"selectedWishEquipQuality"`
	SelectedWishPetRarity    string   `json:"selectedWishPetRarity"`
	AutoSellQualities        []string `json:"autoSellQualities"`
	AutoReleaseRarities      []string `json:"autoReleaseRarities"`
}

func (h *GameHandler) GachaDraw(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req gachaDrawRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}

	result, err := h.gachaService.Draw(c.Request.Context(), userID, service.GachaDrawInput{
		GachaType:                req.GachaType,
		Times:                    req.Times,
		WishlistEnabled:          req.WishlistEnabled,
		SelectedWishEquipQuality: req.SelectedWishEquipQuality,
		SelectedWishPetRarity:    req.SelectedWishPetRarity,
		AutoSellQualities:        req.AutoSellQualities,
		AutoReleaseRarities:      req.AutoReleaseRarities,
	})
	if err != nil {
		h.handleGameError(c, err)
		return
	}

	h.respondWithAchievementSync(c, userID, result)
}

type inventorySellEquipmentRequest struct {
	ItemID string `json:"itemId"`
}

func (h *GameHandler) InventorySellEquipment(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req inventorySellEquipmentRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.ItemID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "itemId is required"})
		return
	}

	result, err := h.inventoryService.SellEquipment(c.Request.Context(), userID, req.ItemID)
	if err != nil {
		h.handleGameError(c, err)
		return
	}
	h.respondWithAchievementSync(c, userID, result)
}

type inventoryBatchSellEquipmentRequest struct {
	Quality       string `json:"quality"`
	EquipmentType string `json:"equipmentType"`
}

func (h *GameHandler) InventoryBatchSellEquipment(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req inventoryBatchSellEquipmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}

	result, err := h.inventoryService.BatchSellEquipment(c.Request.Context(), userID, req.Quality, req.EquipmentType)
	if err != nil {
		h.handleGameError(c, err)
		return
	}
	h.respondWithAchievementSync(c, userID, result)
}

type inventoryReleasePetRequest struct {
	ItemID string `json:"itemId"`
}

func (h *GameHandler) InventoryReleasePet(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req inventoryReleasePetRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.ItemID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "itemId is required"})
		return
	}

	result, err := h.inventoryService.ReleasePet(c.Request.Context(), userID, req.ItemID)
	if err != nil {
		h.handleGameError(c, err)
		return
	}
	h.respondWithAchievementSync(c, userID, result)
}

type inventoryBatchReleasePetsRequest struct {
	Rarity string `json:"rarity"`
}

func (h *GameHandler) InventoryBatchReleasePets(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req inventoryBatchReleasePetsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}

	result, err := h.inventoryService.BatchReleasePets(c.Request.Context(), userID, req.Rarity)
	if err != nil {
		h.handleGameError(c, err)
		return
	}
	h.respondWithAchievementSync(c, userID, result)
}

type inventoryUpgradePetRequest struct {
	ItemID string `json:"itemId"`
}

func (h *GameHandler) InventoryUpgradePet(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req inventoryUpgradePetRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.ItemID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "itemId is required"})
		return
	}

	result, err := h.inventoryService.UpgradePet(c.Request.Context(), userID, req.ItemID)
	if err != nil {
		h.handleGameError(c, err)
		return
	}
	h.respondWithAchievementSync(c, userID, result)
}

type inventoryEvolvePetRequest struct {
	ItemID     string `json:"itemId"`
	FoodItemID string `json:"foodItemId"`
}

func (h *GameHandler) InventoryEvolvePet(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req inventoryEvolvePetRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.ItemID == "" || req.FoodItemID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "itemId and foodItemId are required"})
		return
	}

	result, err := h.inventoryService.EvolvePet(c.Request.Context(), userID, req.ItemID, req.FoodItemID)
	if err != nil {
		h.handleGameError(c, err)
		return
	}
	h.respondWithAchievementSync(c, userID, result)
}

type gameUseItemRequest struct {
	ItemID string `json:"itemId"`
}

func (h *GameHandler) GameUseItem(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req gameUseItemRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.ItemID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "itemId is required"})
		return
	}

	result, err := h.inventoryService.UseItem(c.Request.Context(), userID, req.ItemID)
	if err != nil {
		h.handleGameError(c, err)
		return
	}
	h.respondWithAchievementSync(c, userID, result)
}

type dungeonStartRequest struct {
	Difficulty int `json:"difficulty"`
}

func (h *GameHandler) DungeonStart(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req dungeonStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}

	result, err := h.dungeonService.Start(c.Request.Context(), userID, req.Difficulty)
	if err != nil {
		h.handleGameError(c, err)
		return
	}
	h.respondWithAchievementSync(c, userID, result)
}

type dungeonNextTurnRequest struct {
	SelectedOptionID string `json:"selectedOptionId"`
	RefreshOptions   bool   `json:"refreshOptions"`
}

func (h *GameHandler) DungeonNextTurn(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req dungeonNextTurnRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
			return
		}
	}

	result, err := h.dungeonService.NextTurn(c.Request.Context(), userID, service.DungeonTurnInput{
		SelectedOptionID: req.SelectedOptionID,
		RefreshOptions:   req.RefreshOptions,
	})
	if err != nil {
		h.handleGameError(c, err)
		return
	}
	h.respondWithAchievementSync(c, userID, result)
}

type equipmentEquipRequest struct {
	ItemID string `json:"itemId"`
}

func (h *GameHandler) EquipmentEquip(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req equipmentEquipRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.ItemID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "itemId is required"})
		return
	}

	result, err := h.equipmentService.Equip(c.Request.Context(), userID, req.ItemID)
	if err != nil {
		h.handleGameError(c, err)
		return
	}
	h.respondWithAchievementSync(c, userID, result)
}

type equipmentUnequipRequest struct {
	Slot string `json:"slot"`
}

func (h *GameHandler) EquipmentUnequip(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req equipmentUnequipRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Slot == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slot is required"})
		return
	}

	result, err := h.equipmentService.Unequip(c.Request.Context(), userID, req.Slot)
	if err != nil {
		h.handleGameError(c, err)
		return
	}
	h.respondWithAchievementSync(c, userID, result)
}

type equipmentEnhanceRequest struct {
	ItemID string `json:"itemId"`
}

func (h *GameHandler) EquipmentEnhance(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req equipmentEnhanceRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.ItemID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "itemId is required"})
		return
	}

	result, err := h.equipmentService.Enhance(c.Request.Context(), userID, req.ItemID)
	if err != nil {
		h.handleGameError(c, err)
		return
	}
	h.respondWithAchievementSync(c, userID, result)
}

type equipmentReforgeRequest struct {
	ItemID string `json:"itemId"`
}

func (h *GameHandler) EquipmentReforge(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req equipmentReforgeRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.ItemID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "itemId is required"})
		return
	}

	result, err := h.equipmentService.Reforge(c.Request.Context(), userID, req.ItemID)
	if err != nil {
		h.handleGameError(c, err)
		return
	}
	h.respondWithAchievementSync(c, userID, result)
}

func (h *GameHandler) AchievementsList(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	result, err := h.achievementService.List(c.Request.Context(), userID)
	if err != nil {
		h.handleGameError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *GameHandler) AchievementsSync(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	result, err := h.achievementService.Sync(c.Request.Context(), userID)
	if err != nil {
		h.handleGameError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *GameHandler) respondWithAchievementSync(c *gin.Context, userID uuid.UUID, result any) {
	syncResult, err := h.achievementService.Sync(c.Request.Context(), userID)
	if err != nil {
		h.handleGameError(c, err)
		return
	}

	rawResult, err := json.Marshal(result)
	if err != nil {
		c.JSON(http.StatusOK, result)
		return
	}

	merged := make(map[string]any)
	if err := json.Unmarshal(rawResult, &merged); err != nil {
		c.JSON(http.StatusOK, result)
		return
	}

	if syncResult != nil {
		if syncResult.Snapshot != nil {
			merged["snapshot"] = syncResult.Snapshot
		}
		if len(syncResult.NewlyCompleted) > 0 {
			merged["newlyCompletedAchievements"] = syncResult.NewlyCompleted
		}
	}

	c.JSON(http.StatusOK, merged)
}

func (h *GameHandler) handleGameError(c *gin.Context, err error) {
	var insufficientSpiritError *service.InsufficientSpiritError
	if errors.As(err, &insufficientSpiritError) {
		payload := gin.H{
			"error":          "insufficient spirit",
			"requiredSpirit": insufficientSpiritError.Required,
			"currentSpirit":  insufficientSpiritError.Current,
		}
		if insufficientSpiritError.RegenRate > 0 {
			payload["spiritRegenRate"] = insufficientSpiritError.RegenRate
		}
		if insufficientSpiritError.RetryAfterSeconds > 0 {
			payload["retryAfterSeconds"] = insufficientSpiritError.RetryAfterSeconds
			payload["retryAfterMs"] = insufficientSpiritError.RetryAfterSeconds * 1000
		}
		c.JSON(http.StatusBadRequest, payload)
		return
	}

	var breakthroughUnavailableError *service.BreakthroughUnavailableError
	if errors.As(err, &breakthroughUnavailableError) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":               "breakthrough unavailable",
			"requiredCultivation": breakthroughUnavailableError.RequiredCultivation,
			"currentCultivation":  breakthroughUnavailableError.CurrentCultivation,
		})
		return
	}

	var invalidLocationError *service.InvalidLocationError
	if errors.As(err, &invalidLocationError) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "invalid location",
			"locationId": invalidLocationError.LocationID,
		})
		return
	}

	var dungeonInvalidDifficultyError *service.DungeonInvalidDifficultyError
	if errors.As(err, &dungeonInvalidDifficultyError) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "invalid dungeon difficulty",
			"difficulty": dungeonInvalidDifficultyError.Difficulty,
		})
		return
	}

	var dungeonRunNotActiveError *service.DungeonRunNotActiveError
	if errors.As(err, &dungeonRunNotActiveError) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "dungeon run not active",
		})
		return
	}

	var dungeonInvalidOptionError *service.DungeonInvalidOptionError
	if errors.As(err, &dungeonInvalidOptionError) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":    "invalid dungeon option",
			"optionId": dungeonInvalidOptionError.OptionID,
		})
		return
	}

	var dungeonRefreshExhaustedError *service.DungeonRefreshExhaustedError
	if errors.As(err, &dungeonRefreshExhaustedError) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "dungeon refresh exhausted",
		})
		return
	}

	var locationLockedError *service.LocationLockedError
	if errors.As(err, &locationLockedError) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":         "location locked",
			"requiredLevel": locationLockedError.RequiredLevel,
			"currentLevel":  locationLockedError.CurrentLevel,
		})
		return
	}

	var recipeNotFoundError *service.RecipeNotFoundError
	if errors.As(err, &recipeNotFoundError) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":    "recipe not found",
			"recipeId": recipeNotFoundError.RecipeID,
		})
		return
	}

	var recipeLockedError *service.RecipeLockedError
	if errors.As(err, &recipeLockedError) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":    "recipe locked",
			"recipeId": recipeLockedError.RecipeID,
		})
		return
	}

	var insufficientMaterialsError *service.InsufficientMaterialsError
	if errors.As(err, &insufficientMaterialsError) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "insufficient materials",
			"missing": insufficientMaterialsError.Missing,
		})
		return
	}

	var invalidGachaTypeError *service.InvalidGachaTypeError
	if errors.As(err, &invalidGachaTypeError) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":     "invalid gacha type",
			"gachaType": invalidGachaTypeError.GachaType,
		})
		return
	}

	var invalidGachaTimesError *service.InvalidGachaTimesError
	if errors.As(err, &invalidGachaTimesError) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid gacha times",
			"times": invalidGachaTimesError.Times,
		})
		return
	}

	var insufficientSpiritStonesError *service.InsufficientSpiritStonesError
	if errors.As(err, &insufficientSpiritStonesError) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":                "insufficient spirit stones",
			"requiredSpiritStones": insufficientSpiritStonesError.Required,
			"currentSpiritStones":  insufficientSpiritStonesError.Current,
		})
		return
	}

	var petInventoryFullError *service.PetInventoryFullError
	if errors.As(err, &petInventoryFullError) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":       "pet inventory full",
			"limit":       petInventoryFullError.Limit,
			"currentPets": petInventoryFullError.Current,
		})
		return
	}

	var inventoryItemNotFoundError *service.InventoryItemNotFoundError
	if errors.As(err, &inventoryItemNotFoundError) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "inventory item not found",
			"itemId": inventoryItemNotFoundError.ItemID,
		})
		return
	}

	var invalidInventoryItemTypeError *service.InvalidInventoryItemTypeError
	if errors.As(err, &invalidInventoryItemTypeError) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":    "invalid inventory item type",
			"itemId":   invalidInventoryItemTypeError.ItemID,
			"expected": invalidInventoryItemTypeError.Expected,
			"actual":   invalidInventoryItemTypeError.Actual,
		})
		return
	}

	var invalidRarityError *service.InvalidRarityError
	if errors.As(err, &invalidRarityError) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "invalid rarity",
			"rarity": invalidRarityError.Rarity,
		})
		return
	}

	var petEssenceInsufficientError *service.PetEssenceInsufficientError
	if errors.As(err, &petEssenceInsufficientError) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":              "insufficient pet essence",
			"requiredPetEssence": petEssenceInsufficientError.Required,
			"currentPetEssence":  petEssenceInsufficientError.Current,
		})
		return
	}

	var petEvolveInvalidFoodError *service.PetEvolveInvalidFoodError
	if errors.As(err, &petEvolveInvalidFoodError) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid evolve food pet",
			"message": petEvolveInvalidFoodError.Message,
		})
		return
	}

	var equipmentItemNotFoundError *service.EquipmentItemNotFoundError
	if errors.As(err, &equipmentItemNotFoundError) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "equipment item not found",
			"itemId": equipmentItemNotFoundError.ItemID,
		})
		return
	}

	var equipmentSlotInvalidError *service.EquipmentSlotInvalidError
	if errors.As(err, &equipmentSlotInvalidError) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid equipment slot",
			"slot":  equipmentSlotInvalidError.Slot,
		})
		return
	}

	var equipmentSlotEmptyError *service.EquipmentSlotEmptyError
	if errors.As(err, &equipmentSlotEmptyError) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "equipment slot empty",
			"slot":  equipmentSlotEmptyError.Slot,
		})
		return
	}

	var equipmentRequirementError *service.EquipmentRequirementError
	if errors.As(err, &equipmentRequirementError) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":         "equipment requirement not met",
			"requiredLevel": equipmentRequirementError.Required,
			"currentLevel":  equipmentRequirementError.Current,
		})
		return
	}

	var reinforceStonesInsufficientError *service.ReinforceStonesInsufficientError
	if errors.As(err, &reinforceStonesInsufficientError) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":                   "insufficient reinforce stones",
			"requiredReinforceStones": reinforceStonesInsufficientError.Required,
			"currentReinforceStones":  reinforceStonesInsufficientError.Current,
		})
		return
	}

	var refinementStonesInsufficientError *service.RefinementStonesInsufficientError
	if errors.As(err, &refinementStonesInsufficientError) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":                    "insufficient refinement stones",
			"requiredRefinementStones": refinementStonesInsufficientError.Required,
			"currentRefinementStones":  refinementStonesInsufficientError.Current,
		})
		return
	}

	var equipmentEnhanceMaxLevelError *service.EquipmentEnhanceMaxLevelError
	if errors.As(err, &equipmentEnhanceMaxLevelError) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":        "equipment max level reached",
			"currentLevel": equipmentEnhanceMaxLevelError.CurrentLevel,
		})
		return
	}

	var equipmentInvalidTypeError *service.EquipmentInvalidTypeError
	if errors.As(err, &equipmentInvalidTypeError) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "invalid equipment type",
			"itemId": equipmentInvalidTypeError.ItemID,
			"type":   equipmentInvalidTypeError.Type,
		})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("game action failed: %v", err)})
}
