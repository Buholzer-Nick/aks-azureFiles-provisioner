package k8s

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"aks-azureFiles-controller/internal/naming"
)

const (
	azureFileCSIDriver = "file.csi.azure.com"
	pvNameHashLength   = 12
)

var ErrInvalidPVInput = errors.New("invalid pv input")

// BuildPV constructs a PersistentVolume that binds to the PVC and Azure File share.
// Invariants: deterministic name/spec for same inputs and no external side effects.
func BuildPV(
	pvc *corev1.PersistentVolumeClaim,
	shareName string,
	resourceGroup string,
	storageAccount string,
	server string,
	reclaimPolicy corev1.PersistentVolumeReclaimPolicy,
) (*corev1.PersistentVolume, error) {
	if pvc == nil {
		return nil, fmt.Errorf("pvc is nil: %w", ErrInvalidPVInput)
	}
	if shareName == "" || resourceGroup == "" || storageAccount == "" {
		return nil, fmt.Errorf("share and account inputs required: %w", ErrInvalidPVInput)
	}
	if pvc.Name == "" || pvc.Namespace == "" || pvc.UID == "" {
		return nil, fmt.Errorf("pvc identity required: %w", ErrInvalidPVInput)
	}

	storage := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	if storage == (resource.Quantity{}) {
		return nil, fmt.Errorf("pvc storage request required: %w", ErrInvalidPVInput)
	}

	pvName := pvNameFor(pvc, shareName, storageAccount, resourceGroup)
	volumeHandle := fmt.Sprintf("%s#%s#%s", resourceGroup, storageAccount, shareName)

	labels := map[string]string{
		"azurefile.yourlab.dev/pvc-namespace": pvc.Namespace,
		"azurefile.yourlab.dev/pvc-name":      pvc.Name,
		"azurefile.yourlab.dev/share-name":    shareName,
	}

	annotations := map[string]string{
		"azurefile.yourlab.dev/volume-handle": volumeHandle,
	}

	return &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        pvName,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: storage.DeepCopy(),
			},
			AccessModes:                   append([]corev1.PersistentVolumeAccessMode(nil), pvc.Spec.AccessModes...),
			PersistentVolumeReclaimPolicy: reclaimPolicy,
			ClaimRef: &corev1.ObjectReference{
				Kind:      "PersistentVolumeClaim",
				Namespace: pvc.Namespace,
				Name:      pvc.Name,
				UID:       pvc.UID,
			},
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:       azureFileCSIDriver,
					VolumeHandle: volumeHandle,
					VolumeAttributes: map[string]string{
						"resourceGroup":  resourceGroup,
						"storageAccount": storageAccount,
						"shareName":      shareName,
						"server":         server,
					},
				},
			},
		},
	}, nil
}

func pvNameFor(pvc *corev1.PersistentVolumeClaim, shareName, storageAccount, resourceGroup string) string {
	base := fmt.Sprintf("%s-%s-%s", pvc.Namespace, pvc.Name, shareName)
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s", storageAccount, resourceGroup, pvc.UID)))
	return fmt.Sprintf("pvc-%s-%s", sanitizeName(base), hex.EncodeToString(hash[:])[:pvNameHashLength])
}

func sanitizeName(value string) string {
	sanitized, err := naming.Sanitize(value)
	if err != nil {
		return "pv"
	}
	return sanitized
}
