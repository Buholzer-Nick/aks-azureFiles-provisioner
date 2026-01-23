package controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"aks-azureFiles-controller/internal/constants"
	"aks-azureFiles-controller/internal/k8s"
	"aks-azureFiles-controller/internal/naming"
)

// handleDeletion cleans up Azure resources and Kubernetes PVs when a PVC is deleted.
// Flow:
// 1. Check if we manage this PVC (if not, just remove finalizer).
// 2. Identify the share name (from annotation or computed).
// 3. Delete the Azure Share (unless 'retain-share' annotation is present).
// 4. Delete the PV.
// 5. Remove the Finalizer to allow PVC deletion to complete.
func (r *PVCReconciler) handleDeletion(ctx context.Context, logger logr.Logger, pvc *corev1.PersistentVolumeClaim) (reconcile.Result, error) {
	// 1. Check management
	managed := k8s.IsManagedPVC(pvc)
	if !managed {
		return r.removeFinalizer(ctx, pvc)
	}

	// 2. Identify Share Name
	shareName := ""
	if pvc.Annotations != nil {
		shareName = pvc.Annotations[constants.ShareNameAnnotation]
	}
	if shareName == "" {
		shareName, _ = naming.ComputeShareName(pvc.Namespace, pvc.Name, "")
	}

	logger = logger.WithValues("share", shareName)
	logger.Info("cleanup started")
	r.Recorder.Event(pvc, corev1.EventTypeNormal, constants.EventCleanupStarted, "Cleanup started for Azure File share")

	// 3. Delete Azure Share
	if shouldRetainShare(pvc) {
		r.Recorder.Event(pvc, corev1.EventTypeNormal, constants.EventShareRetained, "Azure File share retained")
	} else if r.Shares != nil && shareName != "" {
		if err := r.Shares.DeleteShare(ctx, shareName); err != nil {
			r.Recorder.Event(pvc, corev1.EventTypeWarning, constants.EventShareError, "Failed to delete Azure File share")
			return reconcile.Result{}, fmt.Errorf("delete share: %w", err)
		}
		r.Recorder.Event(pvc, corev1.EventTypeNormal, constants.EventShareDeleted, "Azure File share deleted")
	}

	// 4. Delete PV
	if err := r.deletePV(ctx, pvc); err != nil {
		return reconcile.Result{}, fmt.Errorf("delete pv: %w", err)
	}

	// 5. Remove Finalizer
	r.Recorder.Event(pvc, corev1.EventTypeNormal, constants.EventCleanupComplete, "Cleanup complete")
	return r.removeFinalizer(ctx, pvc)
}

func (r *PVCReconciler) deletePV(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
	if pvc == nil {
		return nil
	}
	shareName := ""
	if pvc.Annotations != nil {
		shareName = pvc.Annotations[constants.ShareNameAnnotation]
	}
	if shareName == "" {
		return nil
	}
	pv, err := k8s.BuildPV(pvc, shareName, r.Config.ResourceGroup, r.Config.StorageAccount, r.Config.Server, corev1.PersistentVolumeReclaimDelete)
	if err != nil {
		return fmt.Errorf("build pv: %w", err)
	}

	existing := &corev1.PersistentVolume{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: pv.Name}, existing); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get pv: %w", err)
	}
	if !pvMatches(existing, pvc, shareName) {
		return nil
	}
	if err := r.Client.Delete(ctx, existing); err != nil {
		return fmt.Errorf("delete pv: %w", err)
	}
	return nil
}

func (r *PVCReconciler) removeFinalizer(ctx context.Context, pvc *corev1.PersistentVolumeClaim) (reconcile.Result, error) {
	kept := pvc.Finalizers[:0]
	for _, f := range pvc.Finalizers {
		if f != constants.FinalizerName {
			kept = append(kept, f)
		}
	}
	if len(kept) == len(pvc.Finalizers) {
		return reconcile.Result{}, nil
	}

	patch := client.MergeFrom(pvc.DeepCopy())
	pvc.Finalizers = kept
	if err := r.Client.Patch(ctx, pvc, patch); err != nil {
		return reconcile.Result{}, fmt.Errorf("remove finalizer: %w", err)
	}
	return reconcile.Result{}, nil
}

func shouldRetainShare(pvc *corev1.PersistentVolumeClaim) bool {
	if pvc == nil || pvc.Annotations == nil {
		return false
	}
	return pvc.Annotations[constants.RetainShareAnnotation] == "true"
}
