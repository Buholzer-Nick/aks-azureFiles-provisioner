package k8s

import (
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var ErrInvalidPVCRequest = errors.New("invalid pvc request")

// QuotaGiBFromPVC calculates the required storage quota in GiB from a PVC's resource requests.
// It rounds up to the nearest GiB.
func QuotaGiBFromPVC(pvc *corev1.PersistentVolumeClaim) (int32, error) {
	if pvc == nil {
		return 0, fmt.Errorf("pvc is nil: %w", ErrInvalidPVCRequest)
	}
	storage := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	if storage == (resource.Quantity{}) {
		return 0, fmt.Errorf("storage request missing: %w", ErrInvalidPVCRequest)
	}

	bytes := storage.Value()
	if bytes <= 0 {
		return 0, fmt.Errorf("storage request invalid: %w", ErrInvalidPVCRequest)
	}

	const gib = int64(1024 * 1024 * 1024)
	quota := (bytes + gib - 1) / gib
	if quota > int64(^uint32(0)>>1) {
		return 0, fmt.Errorf("storage request too large: %w", ErrInvalidPVCRequest)
	}
	return int32(quota), nil
}
