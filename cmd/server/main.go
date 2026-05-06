package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"ssl-expire-checker/internal/auth"
	"ssl-expire-checker/internal/config"
	"ssl-expire-checker/internal/db"
	"ssl-expire-checker/internal/handlers"
	"ssl-expire-checker/internal/scheduler"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.SupabaseDBURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	dh := &handlers.Domains{Pool: pool, Cfg: cfg}

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.GET("/", func(c *gin.Context) {
		c.File("web/index.html")
	})
	r.Static("/static", "web")

	r.GET("/api/health", handlers.Health)
	r.GET("/api/config", handlers.PublicConfig(cfg))

	authed := r.Group("/api")
	authed.Use(auth.Middleware(cfg.SupabaseJWTSecret))
	authed.GET("/domains", dh.List)
	authed.POST("/domains", dh.Add)
	authed.DELETE("/domains/:id", dh.Delete)
	authed.POST("/domains/:id/scan", dh.ScanOne)
	authed.POST("/scan-all", dh.ScanAll)

	schedCtx, schedCancel := context.WithCancel(context.Background())
	defer schedCancel()
	go scheduler.Loop(schedCtx, pool, cfg)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
	}

	go func() {
		log.Printf("listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	schedCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
