package main

import (
	"context"
	"flag"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wyw14/cry-126/internal/api"
	"github.com/wyw14/cry-126/internal/service"
)

func main() {
	address := flag.String("addr", "127.0.0.1:21226", "listen address")
	dataDir := flag.String("data", "./data", "persistent data directory")
	webDir := flag.String("web", "./web", "web asset directory")
	flag.Parse()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	runtime, err := service.NewRuntime(ctx, *dataDir, time.Now())
	if err != nil {
		logger.Error("runtime initialization failed", "error", err)
		os.Exit(1)
	}
	web := fs.FS(os.DirFS(*webDir))
	handler := api.NewServer(runtime, web).Router()
	server := &http.Server{
		Addr:              *address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	exited := make(chan error, 1)
	go func() {
		logger.Info("TideShield listening", "address", *address)
		exited <- server.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			logger.Error("HTTP shutdown failed", "error", err)
		}
	case err := <-exited:
		if err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server failed", "error", err)
		}
	}
	if err := runtime.Close(); err != nil {
		logger.Error("runtime snapshot failed", "error", err)
		os.Exit(1)
	}
}
