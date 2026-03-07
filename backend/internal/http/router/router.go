package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kowming/vue-idle-xiuxian/backend/internal/http/handler"
	"github.com/kowming/vue-idle-xiuxian/backend/internal/http/middleware"
	"github.com/kowming/vue-idle-xiuxian/backend/internal/service"
)

type Dependencies struct {
	TokenService           *service.TokenService
	PassiveProgressService *service.PassiveProgressService
	GameRealtimeHandler    *handler.GameRealtimeHandler
	AuthHandler            *handler.AuthHandler
	PlayerHandler          *handler.PlayerHandler
	GameHandler            *handler.GameHandler
	RankingHandler         *handler.RankingHandler
	AuctionHandler         *handler.AuctionHandler
	ChatHandler            *handler.ChatHandler
	RechargeHandler        *handler.RechargeHandler
	AdminHandler           *handler.AdminHandler
}

func New(deps Dependencies) *gin.Engine {
	engine := gin.New()
	engine.Use(gin.Recovery(), gin.Logger(), corsMiddleware())

	api := engine.Group("/api/v1")

	api.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	auth := api.Group("/auth")
	{
		auth.GET("/linux-do/authorize", deps.AuthHandler.LinuxDoAuthorize)
		auth.GET("/linux-do/callback", deps.AuthHandler.LinuxDoCallback)
		auth.POST("/dev/login", deps.AuthHandler.DevLogin)
		auth.POST("/refresh", deps.AuthHandler.Refresh)
		auth.POST("/logout", deps.AuthHandler.Logout)
	}

	api.GET("/chat/connect", deps.ChatHandler.Connect)
	api.GET("/game/realtime/connect", deps.GameRealtimeHandler.Connect)
	api.GET("/recharge/callback/credit-linux-do", deps.RechargeHandler.CreditLinuxDoCallback)
	api.POST("/recharge/callback/credit-linux-do", deps.RechargeHandler.CreditLinuxDoCallback)

	authed := api.Group("/")
	authed.Use(middleware.Auth(deps.TokenService), middleware.PassiveProgress(deps.PassiveProgressService))
	{
		authed.GET("/auth/me", deps.AuthHandler.Me)
		authed.GET("/player/snapshot", deps.PlayerHandler.Snapshot)
		authed.GET("/player/active-count", deps.PlayerHandler.ActiveCount)
		authed.GET("/player/public-profile", deps.PlayerHandler.PublicProfile)
		authed.GET("/rankings", deps.RankingHandler.Rankings)
		authed.GET("/rankings/friends", deps.RankingHandler.RankingFriends)
		authed.GET("/rankings/self", deps.RankingHandler.RankingSelf)
		authed.GET("/rankings/follows", deps.RankingHandler.Follows)
		authed.POST("/rankings/follows", deps.RankingHandler.Follow)
		authed.DELETE("/rankings/follows", deps.RankingHandler.Unfollow)
		authed.GET("/auction/list", deps.AuctionHandler.List)
		authed.POST("/auction/create", deps.AuctionHandler.Create)
		authed.POST("/auction/cancel", deps.AuctionHandler.Cancel)
		authed.POST("/auction/buy", deps.AuctionHandler.Buy)
		authed.GET("/auction/my-orders", deps.AuctionHandler.MyOrders)
		authed.GET("/chat/history", deps.ChatHandler.History)
		authed.GET("/chat/mute-status", deps.ChatHandler.MuteStatus)
		authed.POST("/chat/report", deps.ChatHandler.Report)
		authed.GET("/chat/admin/mutes", deps.ChatHandler.AdminMutes)
		authed.POST("/chat/admin/mute", deps.ChatHandler.AdminMute)
		authed.POST("/chat/admin/unmute", deps.ChatHandler.AdminUnmute)
		authed.GET("/chat/admin/reports", deps.ChatHandler.AdminReports)
		authed.POST("/chat/admin/reports/review", deps.ChatHandler.AdminReviewReport)
		authed.GET("/chat/admin/block-words", deps.ChatHandler.AdminBlockWords)
		authed.POST("/chat/admin/block-words", deps.ChatHandler.AdminUpsertBlockWord)
		authed.DELETE("/chat/admin/block-words", deps.ChatHandler.AdminDeleteBlockWord)
		authed.GET("/admin/me", deps.AdminHandler.Me)
		authed.GET("/admin/users", deps.AdminHandler.ListUsers)
		authed.POST("/admin/users", deps.AdminHandler.UpsertUser)
		authed.DELETE("/admin/users", deps.AdminHandler.DeleteUser)
		authed.GET("/admin/runtime-configs", deps.AdminHandler.RuntimeConfigs)
		authed.GET("/admin/runtime-config-audits", deps.AdminHandler.RuntimeConfigAudits)
		authed.POST("/admin/runtime-configs", deps.AdminHandler.RuntimeConfigUpsert)
		authed.GET("/recharge/products", deps.RechargeHandler.Products)
		authed.GET("/recharge/orders", deps.RechargeHandler.Orders)
		authed.POST("/recharge/orders", deps.RechargeHandler.CreateOrder)
		authed.POST("/recharge/orders/mock-paid", deps.RechargeHandler.MockPaid)
		authed.POST("/recharge/orders/sync", deps.RechargeHandler.SyncOrder)
		authed.GET("/game/meditation/status", deps.GameHandler.MeditationStatus)
		authed.POST("/game/meditation/start", deps.GameHandler.MeditationStart)
		authed.POST("/game/meditation/stop", deps.GameHandler.MeditationStop)
		authed.POST("/game/cultivation/once", deps.GameHandler.CultivationOnce)
		authed.POST("/game/cultivation/until-breakthrough", deps.GameHandler.CultivationUntilBreakthrough)
		authed.GET("/game/hunting/maps", deps.GameHandler.HuntingMaps)
		authed.GET("/game/hunting/status", deps.GameHandler.HuntingStatus)
		authed.POST("/game/hunting/start", deps.GameHandler.HuntingStart)
		authed.POST("/game/hunting/tick", deps.GameHandler.HuntingTick)
		authed.POST("/game/hunting/stop", deps.GameHandler.HuntingStop)
		authed.POST("/game/hunting/fight", deps.GameHandler.HuntingFight)
		authed.POST("/game/breakthrough", deps.GameHandler.Breakthrough)
		authed.GET("/game/exploration/status", deps.GameHandler.ExplorationStatus)
		authed.POST("/game/exploration/start", deps.GameHandler.ExplorationStart)
		authed.POST("/game/exploration/auto/start", deps.GameHandler.ExplorationAutoStart)
		authed.POST("/game/exploration/auto/stop", deps.GameHandler.ExplorationAutoStop)
		authed.POST("/game/alchemy/craft", deps.GameHandler.AlchemyCraft)
		authed.POST("/game/gacha/draw", deps.GameHandler.GachaDraw)
		authed.POST("/game/inventory/equipment/sell", deps.GameHandler.InventorySellEquipment)
		authed.POST("/game/inventory/equipment/sell-batch", deps.GameHandler.InventoryBatchSellEquipment)
		authed.POST("/game/inventory/pet/release", deps.GameHandler.InventoryReleasePet)
		authed.POST("/game/inventory/pet/release-batch", deps.GameHandler.InventoryBatchReleasePets)
		authed.POST("/game/inventory/pet/upgrade", deps.GameHandler.InventoryUpgradePet)
		authed.POST("/game/inventory/pet/evolve", deps.GameHandler.InventoryEvolvePet)
		authed.POST("/game/item/use", deps.GameHandler.GameUseItem)
		authed.POST("/game/dungeon/start", deps.GameHandler.DungeonStart)
		authed.POST("/game/dungeon/next-turn", deps.GameHandler.DungeonNextTurn)
		authed.POST("/game/inventory/equipment/equip", deps.GameHandler.EquipmentEquip)
		authed.POST("/game/inventory/equipment/unequip", deps.GameHandler.EquipmentUnequip)
		authed.POST("/game/inventory/equipment/enhance", deps.GameHandler.EquipmentEnhance)
		authed.POST("/game/inventory/equipment/reforge", deps.GameHandler.EquipmentReforge)
		authed.GET("/game/achievements", deps.GameHandler.AchievementsList)
		authed.POST("/game/achievements/sync", deps.GameHandler.AchievementsSync)
	}

	return engine
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			origin = "*"
		}
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Idempotency-Key")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
