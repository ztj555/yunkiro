package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ztj555/yunkiro/relay/internal/handler"
	"github.com/ztj555/yunkiro/relay/internal/hub"
	"github.com/ztj555/yunkiro/relay/internal/logger"
)

func main() {
	port := flag.Int("port", 8080, "port to listen on")
	logLevel := flag.String("log-level", "info", "log level (debug/info/warn/error)")
	flag.Parse()

	log := logger.New(logger.ParseLevel(*logLevel))
	log.Info("Starting relay server on port %d (log-level=%s)", *port, *logLevel)

	h := hub.New(log)
	h.Start()

	wsHandler := handler.New(h, log)

	mux := http.NewServeMux()
	mux.Handle("/ws", wsHandler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		groups, conns := h.Stats()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","groups":%d,"connections":%d}`, groups, conns)
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: mux,
	}

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Server error: %v", err)
			os.Exit(1)
		}
	}()

	log.Info("Relay server is ready")

	<-sigCh
	log.Info("Shutting down...")

	h.Stop()
	h.CloseAll()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Error("Shutdown error: %v", err)
	}

	groups, conns := h.Stats()
	log.Info("Final stats: groups=%d connections=%d", groups, conns)
	log.Info("Relay server stopped")
}
