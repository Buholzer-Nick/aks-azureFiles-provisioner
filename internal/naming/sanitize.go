package naming

import (
	"errors"
	"strings"
)

var ErrInvalidShareName = errors.New("invalid share name")

// Sanitize normalizes a share name to lowercase ASCII with hyphen separators.
// Invariants: output contains only [a-z0-9-] and has no leading/trailing hyphens.
func Sanitize(value string) (string, error) {
	if value == "" {
		return "", ErrInvalidShareName
	}

	var b strings.Builder
	b.Grow(len(value))
	lastWasDash := false

	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastWasDash = false
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
			lastWasDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastWasDash = false
		default:
			if !lastWasDash {
				b.WriteRune('-')
				lastWasDash = true
			}
		}
	}

	sanitized := strings.Trim(b.String(), "-")
	if sanitized == "" {
		return "", ErrInvalidShareName
	}

	return sanitized, nil
}
