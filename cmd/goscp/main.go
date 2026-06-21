// Command goscp is a self-contained, embeddable HTTP file-exchange server —
// a WinSCP-style tool that exchanges files over HTTP with a single binary.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"goscp/internal/config"
	"goscp/internal/server"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		log.Fatalf("config: %v", err)
	}

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("server init: %v", err)
	}

	banner(cfg)

	// Run the listener and wait for either a fatal error or a shutdown signal.
	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		log.Fatalf("server error: %v", err)
	case <-stop:
		log.Println("shutting down…")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatalf("graceful shutdown failed: %v", err)
		}
		log.Println("bye")
	}
}

func banner(cfg *config.Config) {
	host := cfg.Host
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	url := fmt.Sprintf("http://%s:%d", host, cfg.Port)
	fmt.Println("┌──────────────────────────────────────────────┐")
	fmt.Printf("│  goSCP %-39s│\n", version)
	fmt.Println("│  HTTP file exchange — single offline binary    │")
	fmt.Println("├──────────────────────────────────────────────┤")
	fmt.Printf("│  Listening : %-34s│\n", truncate(url, 34))
	fmt.Printf("│  Root dir  : %-34s│\n", truncate(cfg.Root, 34))
	fmt.Println("└──────────────────────────────────────────────┘")
	// Print the token in full on its own line so it is never truncated.
	fmt.Printf("  Access token: %s\n\n", cfg.Token)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
