package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.LeaderElectionEnabled {
		t.Fatalf("LeaderElectionEnabled = false, want true")
	}
	if cfg.LeaderElectionID != defaultLeaderElectionID {
		t.Fatalf("LeaderElectionID = %q, want %q", cfg.LeaderElectionID, defaultLeaderElectionID)
	}
	if cfg.MetricsAddr != defaultMetricsAddr {
		t.Fatalf("MetricsAddr = %q, want %q", cfg.MetricsAddr, defaultMetricsAddr)
	}
	if cfg.HealthAddr != defaultHealthAddr {
		t.Fatalf("HealthAddr = %q, want %q", cfg.HealthAddr, defaultHealthAddr)
	}
	if cfg.SubscriptionID != "" {
		t.Fatalf("SubscriptionID = %q, want empty", cfg.SubscriptionID)
	}
	if cfg.ResourceGroup != "" {
		t.Fatalf("ResourceGroup = %q, want empty", cfg.ResourceGroup)
	}
	if cfg.StorageAccount != "" {
		t.Fatalf("StorageAccount = %q, want empty", cfg.StorageAccount)
	}
	if cfg.Server != "" {
		t.Fatalf("Server = %q, want empty", cfg.Server)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("LEADER_ELECTION_ENABLED", "true")
	t.Setenv("LEADER_ELECTION_ID", "custom-id")
	t.Setenv("METRICS_ADDR", ":9090")
	t.Setenv("HEALTH_ADDR", ":9091")
	t.Setenv("AZURE_SUBSCRIPTION_ID", "sub")
	t.Setenv("AZURE_RESOURCE_GROUP", "rg")
	t.Setenv("AZURE_STORAGE_ACCOUNT", "acct")
	t.Setenv("AZURE_FILE_SERVER", "server")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.LeaderElectionEnabled {
		t.Fatalf("LeaderElectionEnabled = false, want true")
	}
	if cfg.LeaderElectionID != "custom-id" {
		t.Fatalf("LeaderElectionID = %q, want %q", cfg.LeaderElectionID, "custom-id")
	}
	if cfg.MetricsAddr != ":9090" {
		t.Fatalf("MetricsAddr = %q, want %q", cfg.MetricsAddr, ":9090")
	}
	if cfg.HealthAddr != ":9091" {
		t.Fatalf("HealthAddr = %q, want %q", cfg.HealthAddr, ":9091")
	}
	if cfg.SubscriptionID != "sub" {
		t.Fatalf("SubscriptionID = %q, want %q", cfg.SubscriptionID, "sub")
	}
	if cfg.ResourceGroup != "rg" {
		t.Fatalf("ResourceGroup = %q, want %q", cfg.ResourceGroup, "rg")
	}
	if cfg.StorageAccount != "acct" {
		t.Fatalf("StorageAccount = %q, want %q", cfg.StorageAccount, "acct")
	}
	if cfg.Server != "server" {
		t.Fatalf("Server = %q, want %q", cfg.Server, "server")
	}
}

func TestLoadInvalidBool(t *testing.T) {
	t.Setenv("LEADER_ELECTION_ENABLED", "nope")

	_, err := Load()
	if err == nil {
		t.Fatalf("Load() error = nil, want error")
	}
}
