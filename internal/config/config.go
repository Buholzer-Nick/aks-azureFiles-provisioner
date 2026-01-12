package config

import (
	"fmt"
	"os"
	"strconv"
)

const (
	defaultLeaderElectionID = "azurefile-provisioner-leader"
	defaultMetricsAddr      = ":8080"
	defaultHealthAddr       = ":8081"
)

// Config holds runtime configuration loaded from the environment.
type Config struct {
	LeaderElectionEnabled bool
	LeaderElectionID      string
	MetricsAddr           string
	HealthAddr            string
	SubscriptionID        string
	ResourceGroup         string
	StorageAccount        string
	Server                string
}

// Load reads configuration from environment variables.
func Load() (Config, error) {
	leaderElection, err := readBoolEnv("LEADER_ELECTION_ENABLED", true)
	if err != nil {
		return Config{}, fmt.Errorf("read leader election flag: %w", err)
	}

	return Config{
		LeaderElectionEnabled: leaderElection,
		LeaderElectionID:      readEnv("LEADER_ELECTION_ID", defaultLeaderElectionID),
		MetricsAddr:           readEnv("METRICS_ADDR", defaultMetricsAddr),
		HealthAddr:            readEnv("HEALTH_ADDR", defaultHealthAddr),
		SubscriptionID:        readEnv("AZURE_SUBSCRIPTION_ID", ""),
		ResourceGroup:         readEnv("AZURE_RESOURCE_GROUP", ""),
		StorageAccount:        readEnv("AZURE_STORAGE_ACCOUNT", ""),
		Server:                readEnv("AZURE_FILE_SERVER", ""),
	}, nil
}

func readEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func readBoolEnv(key string, fallback bool) (bool, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", key, err)
	}
	return parsed, nil
}
