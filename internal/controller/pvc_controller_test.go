package controller

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"aks-azureFiles-controller/internal/azure"
	"aks-azureFiles-controller/internal/k8s"
	"aks-azureFiles-controller/internal/logging"
	"aks-azureFiles-controller/internal/naming"
)

func TestReconcileCreatesPV(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme corev1: %v", err)
	}
	if err := storagev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme storagev1: %v", err)
	}

	sc := &storagev1.StorageClass{
		ObjectMeta:  metav1.ObjectMeta{Name: "azurefile"},
		Provisioner: k8s.ManagedProvisioner,
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "data",
			Namespace: "team",
			UID:       types.UID("uid-123"),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: stringPtr("azurefile"),
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}

	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sc, pvc).Build()
	shareClient := &azure.FakeShareClient{}

	reconciler := &PVCReconciler{
		Client:   k8sClient,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
		Config: ReconcilerConfig{
			ResourceGroup:  "rg",
			StorageAccount: "account",
			Server:         "server",
		},
		Shares: shareClient,
	}

	ctx := ctrl.LoggerInto(context.Background(), logging.NewLogger())
	_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(pvc)})
	if err != nil {
		t.Fatalf("Reconcile error = %v", err)
	}

	shareName, err := naming.ComputeShareName(pvc.Namespace, pvc.Name, "")
	if err != nil {
		t.Fatalf("ComputeShareName error = %v", err)
	}
	if shareClient.Shares[shareName] != 1 {
		t.Fatalf("Share quota = %d, want 1", shareClient.Shares[shareName])
	}

	expectedPV, err := k8s.BuildPV(pvc, shareName, "rg", "account", "server", corev1.PersistentVolumeReclaimDelete)
	if err != nil {
		t.Fatalf("BuildPV error = %v", err)
	}

	pv := &corev1.PersistentVolume{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: expectedPV.Name}, pv); err != nil {
		t.Fatalf("Get PV error = %v", err)
	}
	if pv.Spec.ClaimRef == nil || pv.Spec.ClaimRef.Name != pvc.Name {
		t.Fatalf("ClaimRef = %#v, want bound to pvc", pv.Spec.ClaimRef)
	}

	updated := &corev1.PersistentVolumeClaim{}
	if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(pvc), updated); err != nil {
		t.Fatalf("Get PVC error = %v", err)
	}
	if updated.Annotations[shareNameAnnotation] != shareName {
		t.Fatalf("share annotation = %q, want %q", updated.Annotations[shareNameAnnotation], shareName)
	}
}

func TestReconcileIdempotent(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme corev1: %v", err)
	}
	if err := storagev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme storagev1: %v", err)
	}

	sc := &storagev1.StorageClass{
		ObjectMeta:  metav1.ObjectMeta{Name: "azurefile"},
		Provisioner: k8s.ManagedProvisioner,
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "data",
			Namespace: "team",
			UID:       types.UID("uid-123"),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: stringPtr("azurefile"),
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}

	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sc, pvc).Build()
	shareClient := &azure.FakeShareClient{}

	reconciler := &PVCReconciler{
		Client:   k8sClient,
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
		Config: ReconcilerConfig{
			ResourceGroup:  "rg",
			StorageAccount: "account",
			Server:         "server",
		},
		Shares: shareClient,
	}

	ctx := ctrl.LoggerInto(context.Background(), logging.NewLogger())
	request := reconcile.Request{NamespacedName: client.ObjectKeyFromObject(pvc)}

	if _, err := reconciler.Reconcile(ctx, request); err != nil {
		t.Fatalf("Reconcile error = %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, request); err != nil {
		t.Fatalf("Reconcile error = %v", err)
	}

	pvList := &corev1.PersistentVolumeList{}
	if err := k8sClient.List(ctx, pvList); err != nil {
		t.Fatalf("List PVs error = %v", err)
	}
	if len(pvList.Items) != 1 {
		t.Fatalf("PV count = %d, want 1", len(pvList.Items))
	}
	if shareClient.EnsureCount[shareNameForTest(pvc)] != 2 {
		t.Fatalf("EnsureShare count = %d, want 2", shareClient.EnsureCount[shareNameForTest(pvc)])
	}
}

func stringPtr(value string) *string {
	return &value
}

func shareNameForTest(pvc *corev1.PersistentVolumeClaim) string {
	name, err := naming.ComputeShareName(pvc.Namespace, pvc.Name, "")
	if err != nil {
		return ""
	}
	return name
}
