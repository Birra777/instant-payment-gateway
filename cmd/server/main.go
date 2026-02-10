package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/ponti/instant-payment-gateway/internal/handler"
	"github.com/ponti/instant-payment-gateway/internal/middleware"
	"github.com/ponti/instant-payment-gateway/internal/repository"
	"github.com/ponti/instant-payment-gateway/internal/service"
	ws "github.com/ponti/instant-payment-gateway/internal/websocket"
)

func main() {
	godotenv.Load()

	// Structured logging.
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	if os.Getenv("ENV") != "production" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	// Database connection.
	dbURL := getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/payment_gateway?sslmode=disable")
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		log.Fatal().Err(err).Msg("failed to ping database")
	}
	log.Info().Msg("connected to database")

	// Initialize layers.
	repo := repository.NewPostgresRepo(pool)
	webhookURL := os.Getenv("WEBHOOK_URL")
	webhookSvc := service.NewWebhookService(webhookURL)
	paymentSvc := service.NewPaymentService(repo, webhookSvc)
	hub := ws.NewHub()

	paymentHandler := handler.NewPaymentHandler(paymentSvc, hub)
	accountHandler := handler.NewAccountHandler(repo)
	stripeWebhookSecret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	momoWebhookSecret := os.Getenv("MOMO_WEBHOOK_SECRET")
	webhookHandler := handler.NewWebhookHandler(stripeWebhookSecret, momoWebhookSecret)

	// Rate limiter: 100 requests/second, burst of 200.
	rateLimiter := middleware.NewRateLimiter(100, 200)
	signingSecret := getEnv("SIGNING_SECRET", "dev-secret-change-me")

	// Router.
	if os.Getenv("ENV") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(middleware.RequestLogger())
	r.Use(middleware.CORS())
	r.Use(gin.Recovery())

	// Health check.
	r.GET("/health", func(c *gin.Context) {
		if err := pool.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status":     "healthy",
			"time":       time.Now().UTC(),
			"ws_clients": hub.ClientCount(),
		})
	})

	// WebSocket endpoint.
	r.GET("/ws", hub.HandleConnect)

	// Public API routes.
	api := r.Group("/api/v1")
	api.Use(rateLimiter.Middleware())
	api.Use(middleware.VerifySignature(signingSecret))
	{
		// Accounts.
		api.POST("/accounts", accountHandler.CreateAccount)
		api.GET("/accounts", accountHandler.ListAccounts)
		api.GET("/accounts/:id", accountHandler.GetAccount)

		// Payments.
		api.POST("/payments", paymentHandler.InitiatePayment)
		api.POST("/payments/:id/authorize", paymentHandler.AuthorizePayment)
		api.POST("/payments/:id/settle", paymentHandler.SettlePayment)
		api.GET("/payments", paymentHandler.ListTransactions)
		api.GET("/payments/:id", paymentHandler.GetTransaction)
		api.GET("/payments/:id/ledger", paymentHandler.GetLedgerEntries)

		// Analytics.
		api.GET("/analytics", paymentHandler.GetAnalytics)

		// Webhooks (incoming from payment providers).
		api.POST("/webhooks/stripe", webhookHandler.HandleStripeWebhook)
		api.POST("/webhooks/momo", webhookHandler.HandleMoMoWebhook)
	}

	// Start server with graceful shutdown.
	port := getEnv("PORT", "8080")
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info().Str("port", port).Msg("starting server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("server forced to shutdown")
	}
	log.Info().Msg("server stopped")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
