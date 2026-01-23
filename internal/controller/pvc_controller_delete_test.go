package controller

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
	"aks-azureFiles-controller/internal/constants"
	"aks-azureFiles-controller/internal/k8s"
	"aks-azureFiles-controller/internal/logging"
	"aks-azureFiles-controller/internal/naming"
)

func TestReconcileAddsFinalizer(t *testing.T) {
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

	pvc := basePVC()
	pvc.Spec.StorageClassName = stringPtr("azurefile")

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

	updated := &corev1.PersistentVolumeClaim{}
	if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(pvc), updated); err != nil {
		t.Fatalf("Get PVC error = %v", err)
	}
	if !containsFinalizer(updated.Finalizers, constants.FinalizerName) {
		t.Fatalf("finalizer missing")
	}
}

func TestReconcileDeletionIdempotent(t *testing.T) {
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

	pvc := basePVC()
	pvc.Spec.StorageClassName = stringPtr("azurefile")
	pvc.Finalizers = []string{constants.FinalizerName}

	now := metav1.NewTime(time.Now())
	pvc.DeletionTimestamp = &now

	shareName, err := naming.ComputeShareName(pvc.Namespace, pvc.Name, "")
	if err != nil {
		t.Fatalf("ComputeShareName error = %v", err)
	}
	pvc.Annotations = map[string]string{constants.ShareNameAnnotation: shareName}

	pv, err := k8s.BuildPV(pvc, shareName, "rg", "account", "server", corev1.PersistentVolumeReclaimDelete)
	if err != nil {
		t.Fatalf("BuildPV error = %v", err)
	}

	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sc, pvc, pv).Build()
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

	updated := &corev1.PersistentVolumeClaim{}
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(pvc), updated)
	if err == nil {
		if containsFinalizer(updated.Finalizers, constants.FinalizerName) {
			t.Fatalf("finalizer not removed")
		}
	} else {
		if !errors.IsNotFound(err) {
			t.Fatalf("Get PVC error = %v", err)
		}
	}

	pvList := &corev1.PersistentVolumeList{}
	if err := k8sClient.List(ctx, pvList); err != nil {
		t.Fatalf("List PVs error = %v", err)
	}
	if len(pvList.Items) != 0 {
		t.Fatalf("PV count = %d, want 0", len(pvList.Items))
	}

	if _, ok := shareClient.Shares[shareName]; ok {
		t.Fatalf("share still present")
	}
}

func basePVC() *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "data",
			Namespace: "team",
			UID:       types.UID("uid-123"),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}
}

func containsFinalizer(finalizers []string, name string) bool {
	for _, finalizer := range finalizers {
		if finalizer == name {
			return true
		}
	}
	return false
}
