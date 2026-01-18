package azure

import (
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

const (
	AuthModeWorkload = "workload"
	AuthModeManaged  = "managed"
	AuthModeEnv      = "env"
)

// CredentialConfig selects the Azure credential type.
type CredentialConfig struct {
	AuthMode string
	TenantID string
	ClientID string
}

// NewCredential creates an Azure TokenCredential based on AuthMode.
func NewCredential(cfg CredentialConfig) (azcore.TokenCredential, string, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.AuthMode))
	if mode == "" {
		mode = AuthModeWorkload
	}

	switch mode {
	case AuthModeWorkload:
		cred, err := azidentity.NewWorkloadIdentityCredential(&azidentity.WorkloadIdentityCredentialOptions{
			TenantID: cfg.TenantID,
			ClientID: cfg.ClientID,
		})
		if err != nil {
			return nil, "", fmt.Errorf("create workload identity credential: %w", err)
		}
		return cred, mode, nil
	case AuthModeManaged:
		options := &azidentity.ManagedIdentityCredentialOptions{}
		if cfg.ClientID != "" {
			options.ID = azidentity.ClientID(cfg.ClientID)
		}
		cred, err := azidentity.NewManagedIdentityCredential(options)
		if err != nil {
			return nil, "", fmt.Errorf("create managed identity credential: %w", err)
		}
		return cred, mode, nil
	case AuthModeEnv:
		cred, err := azidentity.NewEnvironmentCredential(nil)
		if err != nil {
			return nil, "", fmt.Errorf("create environment credential: %w", err)
		}
		return cred, mode, nil
	default:
		return nil, "", fmt.Errorf("unsupported AZURE_AUTH_MODE %q (supported: %s, %s, %s)", cfg.AuthMode, AuthModeWorkload, AuthModeManaged, AuthModeEnv)
	}
}
