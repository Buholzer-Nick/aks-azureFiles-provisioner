package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const ManagedProvisioner = "ylabs.ch/azurefile-share-provisioner"

// IsManagedPVC returns true when the PVC declares a StorageClass reference.
func IsManagedPVC(pvc *corev1.PersistentVolumeClaim) bool {
	if pvc == nil {
		return false
	}
	if pvc.Spec.StorageClassName == nil {
		return false
	}
	return *pvc.Spec.StorageClassName != ""
}

// GetStorageClass loads the StorageClass referenced by the PVC.
func GetStorageClass(ctx context.Context, c client.Client, pvc *corev1.PersistentVolumeClaim) (*storagev1.StorageClass, error) {
	if pvc == nil || pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName == "" {
		return nil, nil
	}

	sc := &storagev1.StorageClass{}
	key := types.NamespacedName{Name: *pvc.Spec.StorageClassName}
	if err := c.Get(ctx, key, sc); err != nil {
		return nil, fmt.Errorf("get storageclass %q: %w", key.Name, err)
	}
	return sc, nil
}

// GetProvisioner returns the provisioner name for the StorageClass.
func GetProvisioner(sc *storagev1.StorageClass) string {
	if sc == nil {
		return ""
	}
	return sc.Provisioner
}
