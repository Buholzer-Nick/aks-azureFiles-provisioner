package azure

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/share"
)

// Client implements ShareClient using Azure SDK for Go.
type Client struct {
	accountName string
	endpoint    string
	credential  azcore.TokenCredential
}

var ErrInvalidShareInput = errors.New("invalid share input")

// NewClient builds a ShareClient with DefaultAzureCredential.
func NewClient(accountName string) (*Client, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("create default credential: %w", err)
	}
	return NewClientWithCredential(accountName, cred)
}

// NewClientWithCredential builds a ShareClient with the provided credential.
func NewClientWithCredential(accountName string, credential azcore.TokenCredential) (*Client, error) {
	if accountName == "" {
		return nil, fmt.Errorf("account name required: %w", ErrInvalidShareInput)
	}
	if credential == nil {
		return nil, fmt.Errorf("credential required: %w", ErrInvalidShareInput)
	}

	endpoint := fmt.Sprintf("https://%s.file.core.windows.net", accountName)
	return &Client{
		accountName: accountName,
		endpoint:    endpoint,
		credential:  credential,
	}, nil
}

// EnsureShare creates the share if it does not already exist.
func (c *Client) EnsureShare(ctx context.Context, shareName string, quotaGiB int32) error {
	if shareName == "" {
		return fmt.Errorf("share name required: %w", ErrInvalidShareInput)
	}
	if quotaGiB < 0 {
		return fmt.Errorf("quota must be non-negative: %w", ErrInvalidShareInput)
	}

	shareClient, err := c.newShareClient(shareName)
	if err != nil {
		return fmt.Errorf("create share client: %w", err)
	}

	var options *share.CreateOptions
	if quotaGiB > 0 {
		options = &share.CreateOptions{Quota: &quotaGiB}
	}

	_, err = shareClient.Create(ctx, options)
	if err != nil {
		if isResponseStatus(err, http.StatusConflict) {
			return nil
		}
		return fmt.Errorf("create share %q: %w", shareName, err)
	}
	return nil
}

// DeleteShare deletes the share if it exists.
func (c *Client) DeleteShare(ctx context.Context, shareName string) error {
	if shareName == "" {
		return fmt.Errorf("share name required: %w", ErrInvalidShareInput)
	}

	shareClient, err := c.newShareClient(shareName)
	if err != nil {
		return fmt.Errorf("create share client: %w", err)
	}

	_, err = shareClient.Delete(ctx, nil)
	if err != nil {
		if isResponseStatus(err, http.StatusNotFound) {
			return nil
		}
		return fmt.Errorf("delete share %q: %w", shareName, err)
	}
	return nil
}

func (c *Client) newShareClient(shareName string) (*share.Client, error) {
	shareURL := fmt.Sprintf("%s/%s", c.endpoint, shareName)
	client, err := share.NewClient(shareURL, c.credential, nil)
	if err != nil {
		return nil, fmt.Errorf("new share client: %w", err)
	}
	return client, nil
}

func isResponseStatus(err error, statusCode int) bool {
	var respErr *azcore.ResponseError
	if errors.As(err, &respErr) {
		return respErr.StatusCode == statusCode
	}
	return false
}
