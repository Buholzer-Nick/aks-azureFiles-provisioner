package naming

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	maxShareNameLength = 63
	hashSuffixLength   = 8
)

// ComputeShareName generates a deterministic Azure File share name.
// Invariants: output is lowercase, uses only [a-z0-9-], and never exceeds maxShareNameLength.
func ComputeShareName(namespace, pvcName string, override string) (string, error) {
	base := override
	if base == "" {
		base = fmt.Sprintf("%s-%s", namespace, pvcName)
	}

	sanitized, err := Sanitize(base)
	if err != nil {
		return "", fmt.Errorf("sanitize share name: %w", err)
	}

	if len(sanitized) <= maxShareNameLength {
		return sanitized, nil
	}

	suffix := "-" + hashString(sanitized)
	maxPrefixLength := maxShareNameLength - len(suffix)
	if maxPrefixLength <= 0 {
		return "", fmt.Errorf("share name too long: %w", ErrInvalidShareName)
	}

	prefix := strings.TrimRight(sanitized[:maxPrefixLength], "-")
	if prefix == "" {
		return "", fmt.Errorf("share name prefix empty: %w", ErrInvalidShareName)
	}

	return prefix + suffix, nil
}

func hashString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:hashSuffixLength]
}
