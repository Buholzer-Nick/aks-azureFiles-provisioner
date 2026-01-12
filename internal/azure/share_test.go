package azure

import (
	"context"
	"errors"
	"testing"
)

func TestFakeShareClientEnsureDelete(t *testing.T) {
	client := &FakeShareClient{}
	ctx := context.Background()

	if err := client.EnsureShare(ctx, "share", 10); err != nil {
		t.Fatalf("EnsureShare error = %v", err)
	}

	if client.Shares["share"] != 10 {
		t.Fatalf("Share quota = %d, want 10", client.Shares["share"])
	}

	if err := client.DeleteShare(ctx, "share"); err != nil {
		t.Fatalf("DeleteShare error = %v", err)
	}
	if _, ok := client.Shares["share"]; ok {
		t.Fatalf("Share still present after delete")
	}
}

func TestFakeShareClientErrors(t *testing.T) {
	ensureErr := errors.New("ensure")
	deleteErr := errors.New("delete")

	client := &FakeShareClient{
		EnsureErr: map[string]error{"share": ensureErr},
		DeleteErr: map[string]error{"share": deleteErr},
	}
	ctx := context.Background()

	if err := client.EnsureShare(ctx, "share", 1); !errors.Is(err, ensureErr) {
		t.Fatalf("EnsureShare error = %v, want %v", err, ensureErr)
	}

	if err := client.DeleteShare(ctx, "share"); !errors.Is(err, deleteErr) {
		t.Fatalf("DeleteShare error = %v, want %v", err, deleteErr)
	}
}
