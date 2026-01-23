package azure

import (
	"context"
	"sync"
)

// FakeShareClient is an in-memory ShareClient for unit tests.
type FakeShareClient struct {
	mu          sync.Mutex
	Shares      map[string]int32
	EnsureErr   map[string]error
	DeleteErr   map[string]error
	EnsureCount map[string]int
}

// EnsureShare records the share creation request in memory.
func (f *FakeShareClient) EnsureShare(_ context.Context, shareName string, quotaGiB int32) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.EnsureErr[shareName]; err != nil {
		return err
	}
	if f.Shares == nil {
		f.Shares = map[string]int32{}
	}
	if f.EnsureCount == nil {
		f.EnsureCount = map[string]int{}
	}
	f.Shares[shareName] = quotaGiB
	f.EnsureCount[shareName]++
	return nil
}

// DeleteShare removes the share entry in memory.
func (f *FakeShareClient) DeleteShare(_ context.Context, shareName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.DeleteErr[shareName]; err != nil {
		return err
	}
	if f.Shares == nil {
		return nil
	}
	delete(f.Shares, shareName)
	return nil
}
