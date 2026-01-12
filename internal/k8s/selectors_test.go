package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestIsManagedPVC(t *testing.T) {
	if IsManagedPVC(nil) {
		t.Fatalf("IsManagedPVC(nil) = true, want false")
	}

	pvc := &corev1.PersistentVolumeClaim{}
	if IsManagedPVC(pvc) {
		t.Fatalf("IsManagedPVC(empty pvc) = true, want false")
	}

	empty := ""
	pvc.Spec.StorageClassName = &empty
	if IsManagedPVC(pvc) {
		t.Fatalf("IsManagedPVC(empty name) = true, want false")
	}

	name := "managed"
	pvc.Spec.StorageClassName = &name
	if !IsManagedPVC(pvc) {
		t.Fatalf("IsManagedPVC(valid name) = false, want true")
	}
}

func TestGetStorageClass(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := storagev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	sc := &storagev1.StorageClass{}
	sc.Name = "managed"
	sc.Provisioner = ManagedProvisioner

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sc).Build()

	pvc := &corev1.PersistentVolumeClaim{}
	pvc.Spec.StorageClassName = stringPtr("managed")

	ctx := context.Background()
	got, err := GetStorageClass(ctx, client, pvc)
	if err != nil {
		t.Fatalf("GetStorageClass error = %v", err)
	}
	if got == nil || got.Name != "managed" {
		t.Fatalf("GetStorageClass = %#v, want name managed", got)
	}
}

func TestGetStorageClassNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := storagev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	pvc := &corev1.PersistentVolumeClaim{}
	pvc.Spec.StorageClassName = stringPtr("missing")

	ctx := context.Background()
	_, err := GetStorageClass(ctx, client, pvc)
	if err == nil {
		t.Fatalf("GetStorageClass error = nil, want error")
	}
	if !errors.IsNotFound(err) {
		t.Fatalf("GetStorageClass error = %v, want NotFound", err)
	}
}

func TestGetStorageClassEmptyName(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := storagev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	pvc := &corev1.PersistentVolumeClaim{}
	pvc.Spec.StorageClassName = stringPtr("")

	ctx := context.Background()
	got, err := GetStorageClass(ctx, client, pvc)
	if err != nil {
		t.Fatalf("GetStorageClass error = %v", err)
	}
	if got != nil {
		t.Fatalf("GetStorageClass = %#v, want nil", got)
	}
}

func TestGetProvisioner(t *testing.T) {
	if got := GetProvisioner(nil); got != "" {
		t.Fatalf("GetProvisioner(nil) = %q, want empty", got)
	}

	sc := &storagev1.StorageClass{}
	sc.Provisioner = ManagedProvisioner
	if got := GetProvisioner(sc); got != ManagedProvisioner {
		t.Fatalf("GetProvisioner(sc) = %q, want %q", got, ManagedProvisioner)
	}
}

func stringPtr(value string) *string {
	return &value
}

func TestGetStorageClassUsesClusterScope(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := storagev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	sc := &storagev1.StorageClass{}
	sc.Name = "cluster"
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sc).Build()

	pvc := &corev1.PersistentVolumeClaim{}
	pvc.Spec.StorageClassName = stringPtr("cluster")

	ctx := context.Background()
	_, err := GetStorageClass(ctx, client, pvc)
	if err != nil {
		t.Fatalf("GetStorageClass error = %v", err)
	}

	key := types.NamespacedName{Name: "cluster"}
	got := &storagev1.StorageClass{}
	if err := client.Get(ctx, key, got); err != nil {
		t.Fatalf("client.Get error = %v", err)
	}
}
