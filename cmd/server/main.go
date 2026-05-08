package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"ssl-expire-checker/internal/auth"
	"ssl-expire-checker/internal/config"
	"ssl-expire-checker/internal/db"
	"ssl-expire-checker/internal/handlers"
	"ssl-expire-checker/internal/scheduler"
	webassets "ssl-expire-checker/web"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	pool, dbErr := db.NewPool(ctx, cfg.SupabaseDBURL)
	if dbErr != nil {
		log.Printf("db disabled: %v", dbErr)
	}
	if pool != nil {
		defer pool.Close()
	}

	dh := &handlers.Domains{Pool: pool, Cfg: cfg}

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	origins := splitOrigins(cfg.FrontendOrigins)
	allowCredentials := !containsWildcardOrigin(origins)
	r.Use(cors.New(cors.Config{
		AllowOrigins:     origins,
		AllowMethods:     []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		AllowCredentials: allowCredentials,
		MaxAge:           12 * time.Hour,
	}))

	webFS := http.FS(webassets.Files)
	r.GET("/", func(c *gin.Context) {
		c.FileFromFS("index.html", webFS)
	})
	r.GET("/app.js", func(c *gin.Context) {
		c.FileFromFS("app.js", webFS)
	})
	r.GET("/config.js", func(c *gin.Context) {
		c.FileFromFS("config.js", webFS)
	})

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
	if pool != nil {
		go scheduler.Loop(schedCtx, pool, cfg)
	} else {
		log.Printf("scheduler disabled: database unavailable")
	}

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

func splitOrigins(v string) []string {
	if strings.TrimSpace(v) == "" {
		return []string{"*"}
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{"*"}
	}
	return out
}

func containsWildcardOrigin(origins []string) bool {
	for _, origin := range origins {
		if origin == "*" {
			return true
		}
	}
	return false
}
