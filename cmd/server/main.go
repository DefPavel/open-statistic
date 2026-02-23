package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"open-statistic/internal/api"
	"open-statistic/internal/database"
	"open-statistic/internal/parser"

	"github.com/gin-gonic/gin"
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustParseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 60 * time.Second
	}
	return d
}

func mustParseInt(s string, fallback int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}

func splitPaths(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func main() {
	dbPath := flag.String("db", getEnv("DB_PATH", "./openstat.db"), "путь к SQLite БД")
	statusPath := flag.String("status", getEnv("STATUS_PATH", "/var/log/openvpn/status.log"), "путь к OpenVPN status-файлу")
	addr := flag.String("addr", ":"+getEnv("PORT", "8080"), "адрес HTTP-сервера")
	interval := flag.Duration("interval", mustParseDuration(getEnv("INTERVAL", "60s")), "интервал сбора статистики")
	retention := flag.Int("retention", mustParseInt(getEnv("RETENTION", "1000"), 1000), "хранить последние N снимков (0 = без ограничения)")
	flag.Parse()

	db, err := database.New(*dbPath)
	if err != nil {
		log.Fatalf("БД: %v", err)
	}
	defer db.Close()

	collect := func(path string) error {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		status, err := parser.ParseBytes(data)
		if err != nil {
			return err
		}
		return db.SaveSnapshot(status)
	}

	h := api.New(db)
	h.SetCollectFn(collect)

	// Первичный сбор
	if _, err := os.Stat(*statusPath); err == nil {
		if err := collect(*statusPath); err != nil {
			log.Printf("Первый сбор: %v", err)
		}
	}

	// Периодический сбор
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		ticker := time.NewTicker(*interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := os.Stat(*statusPath); err == nil {
					if err := collect(*statusPath); err != nil {
						log.Printf("Сбор: %v", err)
					}
					if *retention > 0 {
						db.CleanupOldSnapshots(*retention)
					}
				}
			}
		}
	}()

	apiKey := getEnv("API_KEY", "")
	allowedPaths := []string{"/var/log/openvpn"}
	if p := getEnv("ALLOWED_PATHS", ""); p != "" {
		allowedPaths = splitPaths(p)
	} else if dir := filepath.Dir(*statusPath); dir != "." {
		allowedPaths = append(allowedPaths, dir)
	}
	h.SetAllowedPaths(allowedPaths)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), api.SecurityHeaders())
	r.Use(api.APIKeyAuth(apiKey))

	r.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	r.GET("/users", h.GetUsers)
	r.GET("/users/:name/traffic", h.GetUserTraffic)
	r.GET("/traffic", h.GetAllTraffic)
	r.GET("/connected", h.GetConnected)
	r.POST("/collect", h.CollectNow)

	srv := &http.Server{
		Addr:              *addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Сервер: %v", err)
		}
	}()

	fmt.Printf("Сервер: http://localhost%s\n", *addr)
	fmt.Printf("Status-файл: %s (обновление каждые %s)\n", *statusPath, *interval)
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Shutdown: %v", err)
	}
}
