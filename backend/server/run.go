package server

import (
	"log"
	"net/http"
	"os"
	"time"
)

type Config struct {
	AdminAddr  string
	MockAddr   string
	DBPath     string
	AdminUser  string
	AdminPass  string
	EnableAuth bool
}

func loadConfig() Config {
	adminAddr := os.Getenv("ADMIN_ADDR")
	mockAddr := os.Getenv("MOCK_ADDR")
	if adminAddr == "" {
		adminAddr = "127.0.0.1:8181"
	}
	if mockAddr == "" {
		mockAddr = "127.0.0.1:8180"
	}

	dbPath := getenv("DB_PATH", "data/mock.db")
	adminUser := os.Getenv("ADMIN_USER")
	adminPass := os.Getenv("ADMIN_PASS")
	enableAuth := adminUser != "" && adminPass != ""
	return Config{
		AdminAddr:  adminAddr,
		MockAddr:   mockAddr,
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

	adminRouter := NewAdminRouter(store, cfg)
	mockRouter := NewMockRouter(store)

	adminSrv := &http.Server{
		Addr:              cfg.AdminAddr,
		Handler:           adminRouter,
		ReadHeaderTimeout: 5 * time.Second,
	}

	mockSrv := &http.Server{
		Addr:              cfg.MockAddr,
		Handler:           mockRouter,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 2)

	go func() {
		log.Printf("admin listen on http://%s", cfg.AdminAddr)
		if err := adminSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	go func() {
		log.Printf("mock listen on http://%s", cfg.MockAddr)
		if err := mockSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// 等任一端口启动失败/运行出错即退出（MVP 不做信号优雅退出）。
	if err := <-errCh; err != nil {
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
