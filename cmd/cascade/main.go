package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/amirmatini/cascade/internal/cache"
	"github.com/amirmatini/cascade/internal/config"
	"github.com/amirmatini/cascade/internal/proxy"
)

var (
	configPath = flag.String("config", "config.yaml", "Path to configuration file")
	version    = "0.0.1"
)

func main() {
	flag.Parse()

	log.Printf("Starting Cascade v%s - HTTP/HTTPS Caching Proxy", version)

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Cascade started - Cache: %s (%.3f GB), Buffer: %d KB, TTL: %v",
		cfg.Cache.Directory, cfg.Cache.MaxSizeGB, cfg.Cache.BufferSizeKB, cfg.Cache.DefaultTTL)

	maxSizeBytes := int64(cfg.Cache.MaxSizeGB * 1024 * 1024 * 1024)
	storage, err := cache.NewStorage(
		cfg.Cache.Directory,
		maxSizeBytes,
		cfg.Cache.BufferSizeKB,
		cfg.Cache.MinFileSizeKB,
		cfg.Cache.MaxFileSizeMB,
	)
	if err != nil {
		log.Fatalf("Failed to initialize cache storage: %v", err)
	}

	proxyHandler, err := proxy.New(cfg, storage)
	if err != nil {
		log.Fatalf("Failed to create proxy: %v", err)
	}

	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: proxyHandler,
	}

	go func() {
		log.Printf("Cascade proxy listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Printf("Shutting down gracefully...")
	server.Close()
	log.Printf("Cascade stopped")
}
