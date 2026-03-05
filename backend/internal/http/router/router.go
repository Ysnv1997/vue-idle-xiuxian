package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kowming/vue-idle-xiuxian/backend/internal/http/handler"
	"github.com/kowming/vue-idle-xiuxian/backend/internal/http/middleware"
	"github.com/kowming/vue-idle-xiuxian/backend/internal/service"
)

type Dependencies struct {
	TokenService   *service.TokenService
	AuthHandler    *handler.AuthHandler
	PlayerHandler  *handler.PlayerHandler
	GameHandler    *handler.GameHandler
	RankingHandler *handler.RankingHandler
	AuctionHandler *handler.AuctionHandler
	ChatHandler    *handler.ChatHandler
}

func New(deps Dependencies) *gin.Engine {
	engine := gin.New()
	engine.Use(gin.Recovery(), gin.Logger(), corsMiddleware())

	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	auth := engine.Group("/auth")
	{
		auth.GET("/linux-do/authorize", deps.AuthHandler.LinuxDoAuthorize)
		auth.GET("/linux-do/callback", deps.AuthHandler.LinuxDoCallback)
		auth.POST("/dev/login", deps.AuthHandler.DevLogin)
		auth.POST("/refresh", deps.AuthHandler.Refresh)
		auth.POST("/logout", deps.AuthHandler.Logout)
	}

	engine.GET("/chat/connect", deps.ChatHandler.Connect)

	authed := engine.Group("/")
	authed.Use(middleware.Auth(deps.TokenService))
	{
		authed.GET("/auth/me", deps.AuthHandler.Me)
		authed.GET("/player/snapshot", deps.PlayerHandler.Snapshot)
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
		authed.POST("/auction/bid", deps.AuctionHandler.Bid)
		authed.POST("/auction/accept-bid", deps.AuctionHandler.AcceptBid)
		authed.GET("/auction/my-orders", deps.AuctionHandler.MyOrders)
		authed.GET("/chat/history", deps.ChatHandler.History)
		authed.GET("/chat/mute-status", deps.ChatHandler.MuteStatus)
		authed.POST("/chat/report", deps.ChatHandler.Report)
		authed.GET("/chat/admin/mutes", deps.ChatHandler.AdminMutes)
		authed.POST("/chat/admin/mute", deps.ChatHandler.AdminMute)
		authed.POST("/chat/admin/unmute", deps.ChatHandler.AdminUnmute)
		authed.GET("/chat/admin/block-words", deps.ChatHandler.AdminBlockWords)
		authed.POST("/chat/admin/block-words", deps.ChatHandler.AdminUpsertBlockWord)
		authed.DELETE("/chat/admin/block-words", deps.ChatHandler.AdminDeleteBlockWord)
		authed.POST("/game/cultivation/once", deps.GameHandler.CultivationOnce)
		authed.POST("/game/cultivation/until-breakthrough", deps.GameHandler.CultivationUntilBreakthrough)
		authed.POST("/game/breakthrough", deps.GameHandler.Breakthrough)
		authed.POST("/game/exploration/start", deps.GameHandler.ExplorationStart)
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
