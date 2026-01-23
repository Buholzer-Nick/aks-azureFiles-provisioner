package controller

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"aks-azureFiles-controller/internal/azure"
	"aks-azureFiles-controller/internal/constants"
	"aks-azureFiles-controller/internal/k8s"
	"aks-azureFiles-controller/internal/naming"
)

// handleProvisioning manages the creation lifecycle of an Azure File share and its corresponding Kubernetes PV.
// Flow:
// 1. Validate StorageClass and Provisioner.
// 2. Ensure Finalizer exists on PVC.
// 3. Compute Share Name (honoring overrides).
// 4. Ensure Azure File Share exists (idempotent).
// 5. Ensure Kubernetes PersistentVolume exists and is bound to the share.
// 6. Annotate PVC with the final share name.
func (r *PVCReconciler) handleProvisioning(ctx context.Context, logger logr.Logger, pvc *corev1.PersistentVolumeClaim, outcome *string) (reconcile.Result, error) {
	// 1. Validate StorageClass and Provisioner
	if !k8s.IsManagedPVC(pvc) {
		logger.WithValues("reason", "storageclass missing").Info("skip pvc")
		*outcome = "skip"
		return reconcile.Result{}, nil
	}

	storageClass, err := k8s.GetStorageClass(ctx, r.Client, pvc)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.WithValues("reason", "storageclass not found").Info("skip pvc")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("get storageclass: %w", err)
	}

	provisioner := k8s.GetProvisioner(storageClass)
	if provisioner != k8s.ManagedProvisioner {
		logger.WithValues("reason", "provisioner mismatch", "provisioner", provisioner).Info("skip pvc")
		*outcome = "skip"
		return reconcile.Result{}, nil
	}

	// 2. Ensure Finalizer exists
	if err := r.ensureFinalizer(ctx, pvc); err != nil {
		return reconcile.Result{}, fmt.Errorf("ensure finalizer: %w", err)
	}

	// 3. Compute Share Name
	shareOverride := ""
	if pvc.Annotations != nil {
		shareOverride = pvc.Annotations[constants.ShareOverrideAnnotation]
	}

	shareName, err := naming.ComputeShareName(pvc.Namespace, pvc.Name, shareOverride)
	if err != nil {
		*outcome = "terminal"
		return r.terminalError(logger, pvc, constants.EventShareNameInvalid, fmt.Errorf("compute share name: %w", err))
	}

	quotaGiB, err := k8s.QuotaGiBFromPVC(pvc)
	if err != nil {
		*outcome = "terminal"
		return r.terminalError(logger, pvc, constants.EventPVCInvalid, fmt.Errorf("derive quota: %w", err))
	}

	if r.Shares == nil {
		*outcome = "terminal"
		return r.terminalError(logger, pvc, constants.EventShareClientMissing, fmt.Errorf("share client not configured: %w", ErrInvalidPVCRequest))
	}

	// 4. Ensure Azure File Share
	pvLogger := logger.WithValues("pv", "", "share", shareName)
	r.Recorder.Event(pvc, corev1.EventTypeNormal, constants.EventShareEnsuring, "Ensuring Azure File share exists")
	pvLogger.Info("ensuring share", "quotaGiB", quotaGiB)
	if err := r.Shares.EnsureShare(ctx, shareName, quotaGiB); err != nil {
		r.Recorder.Event(pvc, corev1.EventTypeWarning, constants.EventShareError, "Failed to ensure Azure File share")
		if errors.Is(err, azure.ErrInvalidShareInput) || errors.Is(err, ErrInvalidPVCRequest) {
			*outcome = "terminal"
			return r.terminalError(logger, pvc, constants.EventShareValidation, fmt.Errorf("ensure share: %w", err))
		}
		return reconcile.Result{}, fmt.Errorf("ensure share: %w", err)
	}
	r.Recorder.Event(pvc, corev1.EventTypeNormal, constants.EventShareReady, "Azure File share is ready")

	// 5. Ensure Kubernetes PersistentVolume
	pv, err := k8s.BuildPV(pvc, shareName, r.Config.ResourceGroup, r.Config.StorageAccount, r.Config.Server, corev1.PersistentVolumeReclaimDelete)
	if err != nil {
		*outcome = "terminal"
		return r.terminalError(logger, pvc, constants.EventPVBuildError, fmt.Errorf("build pv: %w", err))
	}

	pvLogger = logger.WithValues("pv", pv.Name, "share", shareName)
	existing := &corev1.PersistentVolume{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: pv.Name}, existing); err != nil {
		if !apierrors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("get pv: %w", err)
		}

		if err := r.Client.Create(ctx, pv); err != nil {
			return reconcile.Result{}, fmt.Errorf("create pv: %w", err)
		}
		r.Recorder.Event(pvc, corev1.EventTypeNormal, constants.EventPVCreated, "Created PersistentVolume for Azure File share")
		pvLogger.Info("created pv")
	} else {
		if !pvMatches(existing, pvc, shareName) {
			r.Recorder.Event(pvc, corev1.EventTypeWarning, constants.EventPVMismatch, "Existing PV does not match expected share or claim")
			return reconcile.Result{}, fmt.Errorf("pv mismatch for %s/%s: %w", pvc.Namespace, pvc.Name, ErrPVMismatch)
		}
		r.Recorder.Event(pvc, corev1.EventTypeNormal, constants.EventPVAlreadyExists, "PersistentVolume already exists")
		pvLogger.Info("pv already exists")
	}

	// 6. Annotate PVC
	if err := r.ensureShareAnnotation(ctx, pvc, shareName); err != nil {
		return reconcile.Result{}, fmt.Errorf("annotate pvc: %w", err)
	}

	return reconcile.Result{}, nil
}

func (r *PVCReconciler) ensureShareAnnotation(ctx context.Context, pvc *corev1.PersistentVolumeClaim, shareName string) error {
	if pvc.Annotations != nil && pvc.Annotations[constants.ShareNameAnnotation] == shareName {
		return nil
	}

	patch := client.MergeFrom(pvc.DeepCopy())
	if pvc.Annotations == nil {
		pvc.Annotations = map[string]string{}
	}
	pvc.Annotations[constants.ShareNameAnnotation] = shareName

	if err := r.Client.Patch(ctx, pvc, patch); err != nil {
		return fmt.Errorf("patch pvc annotations: %w", err)
	}
	return nil
}
