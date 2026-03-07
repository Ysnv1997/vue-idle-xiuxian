package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

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
	realtimeBroker := service.NewGameRealtimeBroker()
	runtimeConfigService := service.NewRuntimeConfigService(pool)
	worldAnnouncementService := service.NewWorldAnnouncementService(runtimeConfigService, realtimeBroker)
	service.SetDefaultWorldAnnouncementService(worldAnnouncementService)
	tokenService := service.NewTokenService(cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	authService := service.NewAuthService(userRepo, tokenService, runtimeConfigService)
	passiveProgressService := service.NewPassiveProgressService(pool, userRepo, runtimeConfigService, realtimeBroker)
	gameService := service.NewGameService(pool, userRepo, runtimeConfigService, realtimeBroker)
	explorationService := service.NewExplorationService(pool, userRepo, realtimeBroker)
	alchemyService := service.NewAlchemyService(pool, userRepo)
	gachaService := service.NewGachaService(pool, userRepo, realtimeBroker)
	inventoryService := service.NewInventoryService(pool, userRepo)
	equipmentService := service.NewEquipmentService(pool, userRepo, realtimeBroker)
	dungeonService := service.NewDungeonService(pool, userRepo)
	achievementService := service.NewAchievementService(pool, userRepo)
	rankingService := service.NewRankingService(pool)
	auctionService := service.NewAuctionService(pool, userRepo)
	chatService := service.NewChatService(pool, userRepo, runtimeConfigService)
	adminService := service.NewAdminService(pool)
	rechargeService := service.NewRechargeService(
		pool,
		userRepo,
		service.RechargeEPayConfig{
			PID:       cfg.RechargeEPayPID,
			Key:       cfg.RechargeEPayKey,
			BaseURL:   cfg.RechargeEPayBaseURL,
			NotifyURL: cfg.RechargeNotifyURL,
			ReturnURL: cfg.RechargeReturnURL,
		},
	)
	if err := runtimeConfigService.EnsureDefaults(ctx); err != nil {
		log.Fatalf("ensure runtime config defaults: %v", err)
	}

	startAuctionSweepWorker(ctx, auctionService, cfg.AuctionSweepTTL, cfg.AuctionSweepMax)
	startHuntingSweepWorker(ctx, passiveProgressService, cfg.HuntingSweepTTL, cfg.HuntingSweepMax)
	startExplorationSweepWorker(ctx, explorationService, cfg.ExplorationSweepTTL, cfg.ExplorationSweepMax)
	startChatCleanupWorker(
		ctx,
		chatService,
		runtimeConfigService,
		cfg.ChatCleanupTTL,
		cfg.ChatRetentionTTL,
		cfg.ChatRetentionMax,
	)
	startDBLockMonitor(
		ctx,
		pool,
		cfg.DBLockMonitorEnabled,
		cfg.DBLockMonitorInterval,
		cfg.DBLockMonitorThreshold,
		cfg.DBLockMonitorMaxRows,
	)

	if err := adminService.EnsureBootstrapAdmins(ctx, cfg.ChatAdminUserIDs); err != nil {
		log.Fatalf("ensure bootstrap admins: %v", err)
	}

	authHandler := handler.NewAuthHandler(cfg, authService, userRepo, adminService)
	playerHandler := handler.NewPlayerHandler(userRepo)
	gameRealtimeHandler := handler.NewGameRealtimeHandler(
		tokenService,
		passiveProgressService,
		gameService,
		explorationService,
		userRepo,
		realtimeBroker,
	)
	rankingHandler := handler.NewRankingHandler(rankingService)
	auctionHandler := handler.NewAuctionHandler(auctionService)
	chatHandler := handler.NewChatHandler(chatService, tokenService, adminService)
	adminHandler := handler.NewAdminHandler(adminService, runtimeConfigService)
	rechargeHandler := handler.NewRechargeHandler(rechargeService, cfg.EnableRechargeMock)
	gameHandler := handler.NewGameHandler(
		gameService,
		explorationService,
		alchemyService,
		gachaService,
		inventoryService,
		equipmentService,
		dungeonService,
		achievementService,
		realtimeBroker,
	)

	engine := httprouter.New(httprouter.Dependencies{
		TokenService:           tokenService,
		PassiveProgressService: passiveProgressService,
		GameRealtimeHandler:    gameRealtimeHandler,
		AuthHandler:            authHandler,
		PlayerHandler:          playerHandler,
		GameHandler:            gameHandler,
		RankingHandler:         rankingHandler,
		AuctionHandler:         auctionHandler,
		ChatHandler:            chatHandler,
		RechargeHandler:        rechargeHandler,
		AdminHandler:           adminHandler,
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

	runCount := 0
	runSweep := func(trigger string) {
		runCount++
		startedAt := time.Now()
		log.Printf(
			"auction sweep #%d start trigger=%s interval=%s batch=%d",
			runCount,
			trigger,
			interval,
			batchSize,
		)
		runCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		result, err := auctionService.SweepExpired(runCtx, batchSize)
		if err != nil {
			log.Printf(
				"auction sweep #%d failed trigger=%s elapsed=%s err=%v",
				runCount,
				trigger,
				time.Since(startedAt),
				err,
			)
			return
		}
		processedOrders := 0
		if result != nil {
			processedOrders = result.ProcessedOrders
		}
		log.Printf(
			"auction sweep #%d done trigger=%s elapsed=%s processed=%d",
			runCount,
			trigger,
			time.Since(startedAt),
			processedOrders,
		)
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		runSweep("startup")
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runSweep("ticker")
			}
		}
	}()
}

func startHuntingSweepWorker(
	ctx context.Context,
	passiveProgressService *service.PassiveProgressService,
	interval time.Duration,
	batchSize int,
) {
	if interval <= 0 {
		interval = time.Second
	}
	if batchSize <= 0 {
		batchSize = 200
	}

	runCount := 0
	runSweep := func(trigger string) {
		runCount++
		startedAt := time.Now()
		log.Printf(
			"hunting sweep #%d start trigger=%s interval=%s batch=%d",
			runCount,
			trigger,
			interval,
			batchSize,
		)
		if ctx.Err() != nil {
			log.Printf("hunting sweep #%d skipped trigger=%s reason=context_done", runCount, trigger)
			return
		}
		runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		processed, err := passiveProgressService.AdvanceActiveRuns(runCtx, batchSize)
		if err != nil {
			if runCtx.Err() != nil || ctx.Err() != nil {
				log.Printf(
					"hunting sweep #%d cancelled trigger=%s elapsed=%s",
					runCount,
					trigger,
					time.Since(startedAt),
				)
				return
			}
			log.Printf(
				"hunting sweep #%d failed trigger=%s elapsed=%s err=%v",
				runCount,
				trigger,
				time.Since(startedAt),
				err,
			)
			return
		}
		log.Printf(
			"hunting sweep #%d done trigger=%s elapsed=%s processed=%d reached_batch_limit=%t",
			runCount,
			trigger,
			time.Since(startedAt),
			processed,
			processed >= batchSize,
		)
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		runSweep("startup")
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runSweep("ticker")
			}
		}
	}()
}

func startExplorationSweepWorker(
	ctx context.Context,
	explorationService *service.ExplorationService,
	interval time.Duration,
	batchSize int,
) {
	if interval <= 0 {
		interval = time.Second
	}
	if batchSize <= 0 {
		batchSize = 200
	}

	runCount := 0
	runSweep := func(trigger string) {
		runCount++
		startedAt := time.Now()
		log.Printf(
			"exploration sweep #%d start trigger=%s interval=%s batch=%d",
			runCount,
			trigger,
			interval,
			batchSize,
		)
		if ctx.Err() != nil {
			log.Printf("exploration sweep #%d skipped trigger=%s reason=context_done", runCount, trigger)
			return
		}
		runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		processed, err := explorationService.AdvanceActiveRuns(runCtx, batchSize)
		if err != nil {
			if runCtx.Err() != nil || ctx.Err() != nil {
				log.Printf(
					"exploration sweep #%d cancelled trigger=%s elapsed=%s",
					runCount,
					trigger,
					time.Since(startedAt),
				)
				return
			}
			log.Printf(
				"exploration sweep #%d failed trigger=%s elapsed=%s err=%v",
				runCount,
				trigger,
				time.Since(startedAt),
				err,
			)
			return
		}
		log.Printf(
			"exploration sweep #%d done trigger=%s elapsed=%s processed=%d reached_batch_limit=%t",
			runCount,
			trigger,
			time.Since(startedAt),
			processed,
			processed >= batchSize,
		)
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		runSweep("startup")
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runSweep("ticker")
			}
		}
	}()
}

func startChatCleanupWorker(
	ctx context.Context,
	chatService *service.ChatService,
	runtimeConfigService *service.RuntimeConfigService,
	interval time.Duration,
	retentionTTL time.Duration,
	maxMessages int,
) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	if retentionTTL <= 0 {
		retentionTTL = 10 * time.Minute
	}
	if maxMessages <= 0 {
		maxMessages = 500
	}

	runCount := 0
	runCleanup := func(trigger string) {
		runCount++
		startedAt := time.Now()
		log.Printf(
			"chat cleanup #%d start trigger=%s interval=%s retention=%s max=%d",
			runCount,
			trigger,
			interval,
			retentionTTL,
			maxMessages,
		)
		runCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		retentionSeconds := int(retentionTTL / time.Second)
		if retentionSeconds <= 0 {
			retentionSeconds = 600
		}
		limitMessages := maxMessages
		if limitMessages <= 0 {
			limitMessages = 500
		}
		if runtimeConfigService != nil {
			retentionSeconds = runtimeConfigService.GetInt(
				runCtx,
				service.RuntimeConfigKeyChatRetentionSeconds,
				retentionSeconds,
				60,
				7*24*60*60,
			)
			limitMessages = runtimeConfigService.GetInt(
				runCtx,
				service.RuntimeConfigKeyChatRetentionMaxMessages,
				limitMessages,
				100,
				50000,
			)
		}

		result, err := chatService.Cleanup(runCtx, time.Duration(retentionSeconds)*time.Second, limitMessages)
		if err != nil {
			log.Printf(
				"chat cleanup #%d failed trigger=%s elapsed=%s err=%v",
				runCount,
				trigger,
				time.Since(startedAt),
				err,
			)
			return
		}
		deletedExpired := int64(0)
		deletedOverflow := int64(0)
		if result != nil {
			deletedExpired = result.DeletedExpired
			deletedOverflow = result.DeletedOverflow
		}
		log.Printf(
			"chat cleanup #%d done trigger=%s elapsed=%s retention_seconds=%d max_messages=%d deleted_expired=%d deleted_overflow=%d",
			runCount,
			trigger,
			time.Since(startedAt),
			retentionSeconds,
			limitMessages,
			deletedExpired,
			deletedOverflow,
		)
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		runCleanup("startup")
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runCleanup("ticker")
			}
		}
	}()
}

func startDBLockMonitor(
	ctx context.Context,
	pool *pgxpool.Pool,
	enabled bool,
	interval time.Duration,
	threshold time.Duration,
	maxRows int,
) {
	if !enabled {
		log.Printf("db lock monitor disabled")
		return
	}
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if threshold <= 0 {
		threshold = 2 * time.Second
	}
	if maxRows <= 0 {
		maxRows = 20
	}

	const query = `
		SELECT
			a.pid,
			COALESCE(a.usename, ''),
			COALESCE(a.application_name, ''),
			COALESCE(a.client_addr::text, ''),
			COALESCE(a.state, ''),
			COALESCE(a.wait_event_type, ''),
			COALESCE(a.wait_event, ''),
			COALESCE(array_to_string(pg_blocking_pids(a.pid), ','), ''),
			EXTRACT(EPOCH FROM (now() - COALESCE(a.query_start, a.state_change, now()))),
			EXTRACT(EPOCH FROM (now() - COALESCE(a.xact_start, a.query_start, a.state_change, now()))),
			LEFT(regexp_replace(COALESCE(a.query, ''), E'[\\n\\r\\t]+', ' ', 'g'), 220)
		FROM pg_stat_activity a
		WHERE a.datname = current_database()
		  AND a.pid <> pg_backend_pid()
		  AND COALESCE(a.state, '') <> 'idle'
		  AND (
			COALESCE(a.wait_event_type, '') = 'Lock'
			OR EXTRACT(EPOCH FROM (now() - COALESCE(a.query_start, a.state_change, now()))) >= $1
			OR EXTRACT(EPOCH FROM (now() - COALESCE(a.xact_start, a.query_start, a.state_change, now()))) >= $1
		  )
		ORDER BY a.query_start NULLS LAST
		LIMIT $2
	`

	runCount := 0
	lastErrLogAt := time.Time{}
	check := func(trigger string) {
		runCount++
		startedAt := time.Now()
		runCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		rows, err := pool.Query(runCtx, query, threshold.Seconds(), maxRows)
		if err != nil {
			if lastErrLogAt.IsZero() || time.Since(lastErrLogAt) >= 30*time.Second {
				log.Printf("db lock monitor #%d failed trigger=%s err=%v", runCount, trigger, err)
				lastErrLogAt = time.Now()
			}
			return
		}
		defer rows.Close()

		hits := 0
		for rows.Next() {
			var (
				pid          int32
				username     string
				application  string
				client       string
				state        string
				waitType     string
				waitEvent    string
				blockingPids string
				queryAgeSec  float64
				xactAgeSec   float64
				sqlPreview   string
			)
			if err := rows.Scan(
				&pid,
				&username,
				&application,
				&client,
				&state,
				&waitType,
				&waitEvent,
				&blockingPids,
				&queryAgeSec,
				&xactAgeSec,
				&sqlPreview,
			); err != nil {
				if lastErrLogAt.IsZero() || time.Since(lastErrLogAt) >= 30*time.Second {
					log.Printf("db lock monitor #%d scan failed trigger=%s err=%v", runCount, trigger, err)
					lastErrLogAt = time.Now()
				}
				return
			}
			hits++
			log.Printf(
				"db lock monitor hit pid=%d user=%s app=%s client=%s state=%s wait=%s/%s blocking=%s query_age=%.2fs xact_age=%.2fs sql=%q",
				pid,
				username,
				application,
				client,
				state,
				waitType,
				waitEvent,
				blockingPids,
				queryAgeSec,
				xactAgeSec,
				sqlPreview,
			)
		}
		if err := rows.Err(); err != nil {
			if lastErrLogAt.IsZero() || time.Since(lastErrLogAt) >= 30*time.Second {
				log.Printf("db lock monitor #%d iterate failed trigger=%s err=%v", runCount, trigger, err)
				lastErrLogAt = time.Now()
			}
			return
		}
		if hits > 0 {
			log.Printf(
				"db lock monitor #%d done trigger=%s elapsed=%s hits=%d threshold=%s",
				runCount,
				trigger,
				time.Since(startedAt),
				hits,
				threshold,
			)
		}
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		check("startup")
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				check("ticker")
			}
		}
	}()
}
