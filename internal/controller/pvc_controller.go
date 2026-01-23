package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"aks-azureFiles-controller/internal/azure"
)

// ReconcilerConfig holds Azure config for the PVC reconciler.
type ReconcilerConfig struct {
	ResourceGroup  string
	StorageAccount string
	Server         string
}

// PVCReconciler reconciles PersistentVolumeClaims for Azure File shares.
// It watches for PVCs that request a specific StorageClass and automatically:
// 1. Provisions an Azure File Share (via Azure SDK).
// 2. Creates a corresponding PersistentVolume (PV) pointing to that share.
// 3. Manages the lifecycle (creation, deletion) of these resources.
type PVCReconciler struct {
	Client   client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	Config   ReconcilerConfig
	Shares   azure.ShareClient
	Metrics  *ReconcileMetrics
}

// Reconcile is idempotent and safe to retry.
func (r *PVCReconciler) Reconcile(ctx context.Context, req reconcile.Request) (result reconcile.Result, err error) {
	start := time.Now()
	outcome := "success"
	defer func() {
		if err != nil {
			outcome = "error"
		}
		if r.Metrics != nil {
			r.Metrics.Observe(outcome, time.Since(start).Seconds())
		}
	}()

	logger := log.FromContext(ctx).WithValues(
		"namespace", req.Namespace,
		"pvc", req.Name,
		"pv", "",
		"share", "",
	)

	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Client.Get(ctx, req.NamespacedName, pvc); err != nil {
		if apierrors.IsNotFound(err) {
			outcome = "skip"
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("get pvc: %w", err)
	}

	if pvc.DeletionTimestamp != nil {
		outcome = "delete"
		return r.handleDeletion(ctx, logger, pvc)
	}

	return r.handleProvisioning(ctx, logger, pvc, &outcome)
}

// SetupWithManager wires the controller into the manager.
func (r *PVCReconciler) SetupWithManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}
