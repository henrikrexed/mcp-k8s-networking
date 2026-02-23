package config

import (
	"os"
	"strconv"
)

type Config struct {
	ClusterName string
	Port        int
	LogLevel    string
	Namespace   string
}

func Load() *Config {
	port := 8080
	if p := os.Getenv("PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}

	clusterName := os.Getenv("CLUSTER_NAME")
	if clusterName == "" {
		clusterName = "unknown"
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	namespace := os.Getenv("NAMESPACE")

	return &Config{
		ClusterName: clusterName,
		Port:        port,
		LogLevel:    logLevel,
		Namespace:   namespace,
	}
}
