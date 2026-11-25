package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"bioskop-mkp/internal/config"
	"bioskop-mkp/internal/handler"
	"bioskop-mkp/internal/middleware"
	"bioskop-mkp/internal/model"
	"bioskop-mkp/internal/repository"
	"bioskop-mkp/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func main() {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("load .env: %v", err)
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}

	jwtExpireHours := 24
	if raw := os.Getenv("JWT_EXPIRE_HOURS"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			log.Fatalf("invalid JWT_EXPIRE_HOURS: %q", raw)
		}
		jwtExpireHours = parsed
	}

	db, err := config.ConnectDB()
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("get sql.DB: %v", err)
	}
	defer sqlDB.Close()

	bookingRepo := repository.NewBookingRepository(db)
	bookingService := service.NewBookingService(bookingRepo)
	router := setupRouter(db, bookingService, jwtSecret, jwtExpireHours)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Background worker to release expired seat locks every 15 seconds.
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if n, err := bookingService.ExpireOverdueBookings(ctx); err != nil {
					log.Printf("auto-expire worker error: %v", err)
				} else if n > 0 {
					log.Printf("auto-restock: %d pesanan kedaluwarsa dilepas kembali", n)
				}
			}
		}
	}()

	srv := &http.Server{
		Addr:              ":" + config.GetEnv("APP_PORT", "8080"),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("server listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
}

func setupRouter(db *gorm.DB, bookingService service.BookingService, jwtSecret string, jwtExpireHours int) *gin.Engine {
	userRepo := repository.NewUserRepository(db)
	scheduleRepo := repository.NewScheduleRepository(db)

	authService := service.NewAuthService(userRepo, jwtSecret, jwtExpireHours)
	scheduleService := service.NewScheduleService(scheduleRepo)

	authHandler := handler.NewAuthHandler(authService)
	scheduleHandler := handler.NewScheduleHandler(scheduleService)
	bookingHandler := handler.NewBookingHandler(bookingService)

	r := gin.Default()
	r.Static("/static", "./web")
	r.GET("/", func(c *gin.Context) {
		if _, err := os.Stat("./web/index.html"); err == nil {
			c.File("./web/index.html")
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"layanan": "API Tiket Bioskop",
			"status":  "berjalan",
			"health":  "/health",
			"api":     "/api/v1",
		})
	})

	r.GET("/health", func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil || sqlDB.PingContext(c.Request.Context()) != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "gangguan", "message": "koneksi database bermasalah"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "server dan database terhubung"})
	})

	v1 := r.Group("/api/v1")

	catalog := v1.Group("/catalog")
	{
		catalog.GET("/metadata", bookingHandler.GetCatalogMetadata)
		catalog.GET("/schedules", scheduleHandler.List)
		catalog.GET("/schedules/:id/seats", bookingHandler.GetScheduleSeats)
	}

	auth := v1.Group("/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
		auth.GET("/me", middleware.AuthMiddleware(jwtSecret), authHandler.Me)
	}

	schedules := v1.Group("/schedules", middleware.AuthMiddleware(jwtSecret))
	{
		schedules.GET("", scheduleHandler.List)
		schedules.GET("/:id", scheduleHandler.GetByID)

		adminSchedules := schedules.Group("", middleware.RequireRole(model.RoleAdmin))
		{
			adminSchedules.POST("", scheduleHandler.Create)
			adminSchedules.PUT("/:id", scheduleHandler.Update)
			adminSchedules.PATCH("/:id/cancel", scheduleHandler.Cancel)
			adminSchedules.DELETE("/:id", scheduleHandler.Delete)
		}
	}

	bookings := v1.Group("/bookings", middleware.AuthMiddleware(jwtSecret))
	{
		bookings.GET("", bookingHandler.ListMyBookings)
		bookings.POST("", bookingHandler.Create)
		bookings.GET("/:id", bookingHandler.GetByID)
		bookings.POST("/:id/pay", bookingHandler.Pay)
		bookings.POST("/:id/cancel", bookingHandler.Cancel)
	}

	return r
}
