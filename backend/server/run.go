package server

import (
	"log"
	"net/http"
	"os"
	"time"
)

type Config struct {
	Addr       string
	DBPath     string
	AdminUser  string
	AdminPass  string
	EnableAuth bool
}

func loadConfig() Config {
	addr := getenv("ADDR", "127.0.0.1:8080")
	dbPath := getenv("DB_PATH", "data/mock.db")
	adminUser := os.Getenv("ADMIN_USER")
	adminPass := os.Getenv("ADMIN_PASS")
	enableAuth := adminUser != "" && adminPass != ""
	return Config{
		Addr:       addr,
		DBPath:     dbPath,
		AdminUser:  adminUser,
		AdminPass:  adminPass,
		EnableAuth: enableAuth,
	}
}

func Run() {
	cfg := loadConfig()
	if err := ensureDir(cfg.DBPath); err != nil {
		log.Fatalf("ensure db dir failed: %v", err)
	}

	db, err := openDB(cfg.DBPath)
	if err != nil {
		log.Fatalf("open db failed: %v", err)
	}
	defer db.Close()

	store, err := NewStore(db)
	if err != nil {
		log.Fatalf("init store failed: %v", err)
	}

	router := NewRouter(store, cfg)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("mock-server listen on %s", cfg.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
}

func getenv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}
