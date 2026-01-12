package k8s

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestBuildPV(t *testing.T) {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "data",
			Namespace: "team",
			UID:       types.UID("uid-123"),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("5Gi"),
				},
			},
		},
	}

	pv, err := BuildPV(pvc, "share", "rg", "account", "server", corev1.PersistentVolumeReclaimDelete)
	if err != nil {
		t.Fatalf("BuildPV error = %v", err)
	}

	if pv.Name == "" {
		t.Fatalf("PV name is empty")
	}
	if pv.Spec.CSI == nil {
		t.Fatalf("PV CSI source is nil")
	}
	if pv.Spec.CSI.Driver != azureFileCSIDriver {
		t.Fatalf("Driver = %q, want %q", pv.Spec.CSI.Driver, azureFileCSIDriver)
	}
	if pv.Spec.CSI.VolumeHandle != "rg#account#share" {
		t.Fatalf("VolumeHandle = %q, want %q", pv.Spec.CSI.VolumeHandle, "rg#account#share")
	}
	if pv.Spec.CSI.VolumeAttributes["resourceGroup"] != "rg" {
		t.Fatalf("resourceGroup = %q, want %q", pv.Spec.CSI.VolumeAttributes["resourceGroup"], "rg")
	}
	if pv.Spec.CSI.VolumeAttributes["storageAccount"] != "account" {
		t.Fatalf("storageAccount = %q, want %q", pv.Spec.CSI.VolumeAttributes["storageAccount"], "account")
	}
	if pv.Spec.CSI.VolumeAttributes["shareName"] != "share" {
		t.Fatalf("shareName = %q, want %q", pv.Spec.CSI.VolumeAttributes["shareName"], "share")
	}
	if pv.Spec.CSI.VolumeAttributes["server"] != "server" {
		t.Fatalf("server = %q, want %q", pv.Spec.CSI.VolumeAttributes["server"], "server")
	}
	if pv.Spec.PersistentVolumeReclaimPolicy != corev1.PersistentVolumeReclaimDelete {
		t.Fatalf("ReclaimPolicy = %q, want %q", pv.Spec.PersistentVolumeReclaimPolicy, corev1.PersistentVolumeReclaimDelete)
	}
	if pv.Spec.ClaimRef == nil || pv.Spec.ClaimRef.Name != "data" || pv.Spec.ClaimRef.Namespace != "team" || pv.Spec.ClaimRef.UID != pvc.UID {
		t.Fatalf("ClaimRef = %#v, want PVC reference", pv.Spec.ClaimRef)
	}
	if pv.Spec.AccessModes[0] != corev1.ReadWriteOnce {
		t.Fatalf("AccessModes = %#v, want ReadWriteOnce", pv.Spec.AccessModes)
	}
	wantStorage := resource.MustParse("5Gi")
	gotStorage := pv.Spec.Capacity[corev1.ResourceStorage]
	if (&gotStorage).Cmp(wantStorage) != 0 {
		t.Fatalf("Capacity = %v, want %v", gotStorage, wantStorage)
	}
	if pv.Labels["azurefile.yourlab.dev/pvc-namespace"] != "team" {
		t.Fatalf("label pvc-namespace = %q, want %q", pv.Labels["azurefile.yourlab.dev/pvc-namespace"], "team")
	}
	if pv.Labels["azurefile.yourlab.dev/pvc-name"] != "data" {
		t.Fatalf("label pvc-name = %q, want %q", pv.Labels["azurefile.yourlab.dev/pvc-name"], "data")
	}
	if pv.Labels["azurefile.yourlab.dev/share-name"] != "share" {
		t.Fatalf("label share-name = %q, want %q", pv.Labels["azurefile.yourlab.dev/share-name"], "share")
	}
}

func TestBuildPVDeterministic(t *testing.T) {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "data",
			Namespace: "team",
			UID:       types.UID("uid-123"),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}

	pv1, err := BuildPV(pvc, "share", "rg", "account", "server", corev1.PersistentVolumeReclaimRetain)
	if err != nil {
		t.Fatalf("BuildPV error = %v", err)
	}
	pv2, err := BuildPV(pvc, "share", "rg", "account", "server", corev1.PersistentVolumeReclaimRetain)
	if err != nil {
		t.Fatalf("BuildPV error = %v", err)
	}
	if pv1.Name != pv2.Name {
		t.Fatalf("PV name mismatch: %q vs %q", pv1.Name, pv2.Name)
	}
	if pv1.Spec.CSI.VolumeHandle != pv2.Spec.CSI.VolumeHandle {
		t.Fatalf("VolumeHandle mismatch: %q vs %q", pv1.Spec.CSI.VolumeHandle, pv2.Spec.CSI.VolumeHandle)
	}
}

func TestBuildPVInvalid(t *testing.T) {
	_, err := BuildPV(nil, "", "", "", "", corev1.PersistentVolumeReclaimDelete)
	if err == nil {
		t.Fatalf("BuildPV error = nil, want error")
	}
}
