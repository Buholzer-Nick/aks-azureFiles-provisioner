package azure

import (
	"context"
)

// ShareClient manages Azure File shares.
type ShareClient interface {
	EnsureShare(ctx context.Context, shareName string, quotaGiB int32) error
	DeleteShare(ctx context.Context, shareName string) error
}
