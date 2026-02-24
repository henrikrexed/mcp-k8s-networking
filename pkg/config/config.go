package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ClusterName         string
	Port                int
	LogLevel            string
	Namespace           string
	CacheTTL            time.Duration
	ToolTimeout         time.Duration
	ProbeNamespace      string
	ProbeImage          string
	MaxConcurrentProbes int
}

func Load() (*Config, error) {
	clusterName := os.Getenv("CLUSTER_NAME")
	if clusterName == "" {
		return nil, fmt.Errorf("CLUSTER_NAME environment variable is required")
	}

	port := 8080
	if p := os.Getenv("PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	namespace := os.Getenv("NAMESPACE")

	cacheTTL := 30 * time.Second
	if v := os.Getenv("CACHE_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cacheTTL = d
		}
	}

	toolTimeout := 10 * time.Second
	if v := os.Getenv("TOOL_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			toolTimeout = d
		}
	}

	probeNamespace := os.Getenv("PROBE_NAMESPACE")
	if probeNamespace == "" {
		probeNamespace = "mcp-diagnostics"
	}

	probeImage := os.Getenv("PROBE_IMAGE")
	if probeImage == "" {
		probeImage = "ghcr.io/mcp-k8s-networking/probe:latest"
	}

	maxProbes := 5
	if v := os.Getenv("MAX_CONCURRENT_PROBES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			if n < 1 {
				n = 1
			} else if n > 20 {
				n = 20
			}
			maxProbes = n
		}
	}

	return &Config{
		ClusterName:         clusterName,
		Port:                port,
		LogLevel:            logLevel,
		Namespace:           namespace,
		CacheTTL:            cacheTTL,
		ToolTimeout:         toolTimeout,
		ProbeNamespace:      probeNamespace,
		ProbeImage:          probeImage,
		MaxConcurrentProbes: maxProbes,
	}, nil
}

// SetupLogging initializes the global slog logger with JSON output at the specified level.
func SetupLogging(level string) {
	var slogLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn", "warning":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel})
	slog.SetDefault(slog.New(handler))
}
