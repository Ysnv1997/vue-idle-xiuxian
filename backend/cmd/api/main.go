package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/kowming/vue-idle-xiuxian/backend/internal/config"
	"github.com/kowming/vue-idle-xiuxian/backend/internal/database"
	"github.com/kowming/vue-idle-xiuxian/backend/internal/http/handler"
	httprouter "github.com/kowming/vue-idle-xiuxian/backend/internal/http/router"
	"github.com/kowming/vue-idle-xiuxian/backend/internal/migration"
	"github.com/kowming/vue-idle-xiuxian/backend/internal/repository"
	"github.com/kowming/vue-idle-xiuxian/backend/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := database.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	if err := migration.Apply(ctx, pool, cfg.MigrationsDir); err != nil {
		log.Fatalf("apply migrations: %v", err)
	}

	userRepo := repository.NewUserRepository(pool)
	tokenService := service.NewTokenService(cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	authService := service.NewAuthService(userRepo, tokenService)
	gameService := service.NewGameService(pool, userRepo)
	explorationService := service.NewExplorationService(pool, userRepo)
	alchemyService := service.NewAlchemyService(pool, userRepo)
	gachaService := service.NewGachaService(pool, userRepo)
	inventoryService := service.NewInventoryService(pool, userRepo)
	equipmentService := service.NewEquipmentService(pool, userRepo)
	dungeonService := service.NewDungeonService(pool, userRepo)
	achievementService := service.NewAchievementService(pool, userRepo)
	rankingService := service.NewRankingService(pool)
	auctionService := service.NewAuctionService(pool, userRepo)
	chatService := service.NewChatService(pool, userRepo)
	startAuctionSweepWorker(ctx, auctionService, cfg.AuctionSweepTTL, cfg.AuctionSweepMax)

	authHandler := handler.NewAuthHandler(cfg, authService, userRepo)
	playerHandler := handler.NewPlayerHandler(userRepo)
	rankingHandler := handler.NewRankingHandler(rankingService)
	auctionHandler := handler.NewAuctionHandler(auctionService)
	chatHandler := handler.NewChatHandler(chatService, tokenService, cfg.ChatAdminUserIDs)
	gameHandler := handler.NewGameHandler(
		gameService,
		explorationService,
		alchemyService,
		gachaService,
		inventoryService,
		equipmentService,
		dungeonService,
		achievementService,
	)

	engine := httprouter.New(httprouter.Dependencies{
		TokenService:   tokenService,
		AuthHandler:    authHandler,
		PlayerHandler:  playerHandler,
		GameHandler:    gameHandler,
		RankingHandler: rankingHandler,
		AuctionHandler: auctionHandler,
		ChatHandler:    chatHandler,
	})

	httpServer := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown server: %v", err)
		}
	}()

	log.Printf("backend listening on %s", cfg.Addr())
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("start server: %v", err)
	}
}

func startAuctionSweepWorker(ctx context.Context, auctionService *service.AuctionService, interval time.Duration, batchSize int) {
	if interval <= 0 {
		interval = time.Minute
	}
	if batchSize <= 0 {
		batchSize = 100
	}

	runSweep := func() {
		runCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		result, err := auctionService.SweepExpired(runCtx, batchSize)
		if err != nil {
			log.Printf("auction sweep failed: %v", err)
			return
		}
		if result != nil && result.ProcessedOrders > 0 {
			log.Printf("auction sweep processed %d expired orders", result.ProcessedOrders)
		}
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		runSweep()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runSweep()
			}
		}
	}()
}
