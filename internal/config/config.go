// Package config holds runtime configuration for the goSCP server.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
)

// Config is the resolved server configuration.
type Config struct {
	// Addr is the TCP address the HTTP server listens on (e.g. ":8080").
	// It is derived from Host and Port unless an explicit --addr is given.
	Addr string
	// Port is the TCP port the server listens on.
	Port int
	// Host is the interface to bind to (empty = all interfaces).
	Host string
	// Root is the absolute path that bounds every file operation.
	Root string
	// Token is the bearer token required by the API. When empty at startup a
	// random one is generated and printed to stdout.
	Token string
	// MaxUploadBytes caps a single multipart upload request.
	MaxUploadBytes int64
}

// Load resolves configuration from flags and environment variables.
// Precedence: command-line flag > environment variable > default.
func Load(args []string) (*Config, error) {
	fs := flag.NewFlagSet("goscp", flag.ContinueOnError)

	port := fs.Int("port", int(envInt("GOSCP_PORT", 8080)), "TCP port to listen on")
	host := fs.String("host", env("GOSCP_HOST", ""), "host/interface to bind (empty = all interfaces)")
	addr := fs.String("addr", env("GOSCP_ADDR", ""), "full listen address (overrides --host/--port), e.g. :8080")
	root := fs.String("root", env("GOSCP_ROOT", "."), "root directory exposed for file exchange")
	token := fs.String("token", env("GOSCP_TOKEN", ""), "bearer token for API auth (auto-generated if empty)")
	maxMB := fs.Int64("max-upload-mb", envInt("GOSCP_MAX_UPLOAD_MB", 2048), "maximum upload size per request in MB")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if *port < 1 || *port > 65535 {
		return nil, fmt.Errorf("port %d is out of range (1-65535)", *port)
	}

	// --addr, when provided, wins over the host/port pair.
	listenAddr := *addr
	if listenAddr == "" {
		listenAddr = net.JoinHostPort(*host, strconv.Itoa(*port))
	}

	absRoot, err := filepath.Abs(*root)
	if err != nil {
		return nil, fmt.Errorf("resolving root: %w", err)
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("root %q is not accessible: %w", absRoot, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("root %q is not a directory", absRoot)
	}

	tok := *token
	if tok == "" {
		tok = randomToken()
	}

	return &Config{
		Addr:           listenAddr,
		Port:           *port,
		Host:           *host,
		Root:           absRoot,
		Token:          tok,
		MaxUploadBytes: *maxMB << 20,
	}, nil
}

func randomToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		// rand.Read never fails on supported platforms; fall back deterministically.
		return "insecure-token-change-me"
	}
	return hex.EncodeToString(b)
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envInt(key string, def int64) int64 {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		var n int64
		if _, err := fmt.Sscan(v, &n); err == nil {
			return n
		}
	}
	return def
}
