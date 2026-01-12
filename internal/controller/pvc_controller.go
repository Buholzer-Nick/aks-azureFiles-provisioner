package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"aks-azureFiles-controller/internal/azure"
	"aks-azureFiles-controller/internal/k8s"
	"aks-azureFiles-controller/internal/naming"
)

const (
	shareOverrideAnnotation = "ylabs.ch/azurefile-share-override"
	shareNameAnnotation     = "ylabs.ch/azurefile-share-name"
	retainShareAnnotation   = "ylabs.ch/retain-share"
	finalizerName           = "ylabs.ch/azurefile-provisioner/finalizer"
	eventShareEnsuring      = "ShareEnsuring"
	eventShareReady         = "ShareReady"
	eventShareError         = "ShareError"
	eventShareValidation    = "ShareValidationError"
	eventShareNameInvalid   = "ShareNameInvalid"
	eventPVCInvalid         = "PVCInvalid"
	eventShareClientMissing = "ShareClientMissing"
	eventPVBuildError       = "PVBuildError"
	eventCleanupStarted     = "CleanupStarted"
	eventShareDeleted       = "ShareDeleted"
	eventShareRetained      = "ShareRetained"
	eventCleanupComplete    = "CleanupComplete"
)

// ReconcilerConfig holds Azure config for the PVC reconciler.
type ReconcilerConfig struct {
	ResourceGroup  string
	StorageAccount string
	Server         string
}

// PVCReconciler reconciles PersistentVolumeClaims for Azure File shares.
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

	if !k8s.IsManagedPVC(pvc) {
		logger.WithValues("reason", "storageclass missing").Info("skip pvc")
		outcome = "skip"
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
		outcome = "skip"
		return reconcile.Result{}, nil
	}

	if err := r.ensureFinalizer(ctx, pvc); err != nil {
		return reconcile.Result{}, fmt.Errorf("ensure finalizer: %w", err)
	}

	shareOverride := ""
	if pvc.Annotations != nil {
		shareOverride = pvc.Annotations[shareOverrideAnnotation]
	}

	shareName, err := naming.ComputeShareName(pvc.Namespace, pvc.Name, shareOverride)
	if err != nil {
		outcome = "terminal"
		return r.terminalError(logger, pvc, eventShareNameInvalid, fmt.Errorf("compute share name: %w", err))
	}

	quotaGiB, err := quotaGiBFromPVC(pvc)
	if err != nil {
		outcome = "terminal"
		return r.terminalError(logger, pvc, eventPVCInvalid, fmt.Errorf("derive quota: %w", err))
	}

	if r.Shares == nil {
		outcome = "terminal"
		return r.terminalError(logger, pvc, eventShareClientMissing, fmt.Errorf("share client not configured: %w", ErrInvalidPVCRequest))
	}

	pvLogger := logger.WithValues("pv", "", "share", shareName)
	r.Recorder.Event(pvc, corev1.EventTypeNormal, eventShareEnsuring, "Ensuring Azure File share exists")
	pvLogger.Info("ensuring share", "quotaGiB", quotaGiB)
	if err := r.Shares.EnsureShare(ctx, shareName, quotaGiB); err != nil {
		r.Recorder.Event(pvc, corev1.EventTypeWarning, eventShareError, "Failed to ensure Azure File share")
		if errors.Is(err, azure.ErrInvalidShareInput) || errors.Is(err, ErrInvalidPVCRequest) {
			outcome = "terminal"
			return r.terminalError(logger, pvc, eventShareValidation, fmt.Errorf("ensure share: %w", err))
		}
		return reconcile.Result{}, fmt.Errorf("ensure share: %w", err)
	}
	r.Recorder.Event(pvc, corev1.EventTypeNormal, eventShareReady, "Azure File share is ready")

	pv, err := k8s.BuildPV(pvc, shareName, r.Config.ResourceGroup, r.Config.StorageAccount, r.Config.Server, corev1.PersistentVolumeReclaimDelete)
	if err != nil {
		outcome = "terminal"
		return r.terminalError(logger, pvc, eventPVBuildError, fmt.Errorf("build pv: %w", err))
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
		r.Recorder.Event(pvc, corev1.EventTypeNormal, "PVCreated", "Created PersistentVolume for Azure File share")
		pvLogger.Info("created pv")
	} else {
		if !pvMatches(existing, pvc, shareName) {
			r.Recorder.Event(pvc, corev1.EventTypeWarning, "PVMismatch", "Existing PV does not match expected share or claim")
			return reconcile.Result{}, fmt.Errorf("pv mismatch for %s/%s: %w", pvc.Namespace, pvc.Name, ErrPVMismatch)
		}
		r.Recorder.Event(pvc, corev1.EventTypeNormal, "PVAlreadyExists", "PersistentVolume already exists")
		pvLogger.Info("pv already exists")
	}

	if err := r.ensureShareAnnotation(ctx, pvc, shareName); err != nil {
		return reconcile.Result{}, fmt.Errorf("annotate pvc: %w", err)
	}

	return reconcile.Result{}, nil
}

// SetupWithManager wires the controller into the manager.
func (r *PVCReconciler) SetupWithManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}

var ErrPVMismatch = errors.New("pv spec mismatch")
var ErrInvalidPVCRequest = errors.New("invalid pvc request")

func pvMatches(pv *corev1.PersistentVolume, pvc *corev1.PersistentVolumeClaim, shareName string) bool {
	if pv == nil || pvc == nil {
		return false
	}
	if pv.Spec.ClaimRef == nil || pv.Spec.ClaimRef.UID != pvc.UID || pv.Spec.ClaimRef.Namespace != pvc.Namespace || pv.Spec.ClaimRef.Name != pvc.Name {
		return false
	}
	if pv.Spec.CSI == nil {
		return false
	}
	if pv.Spec.CSI.VolumeAttributes["shareName"] != shareName {
		return false
	}
	return true
}

func (r *PVCReconciler) ensureShareAnnotation(ctx context.Context, pvc *corev1.PersistentVolumeClaim, shareName string) error {
	if pvc.Annotations != nil && pvc.Annotations[shareNameAnnotation] == shareName {
		return nil
	}

	patch := client.MergeFrom(pvc.DeepCopy())
	if pvc.Annotations == nil {
		pvc.Annotations = map[string]string{}
	}
	pvc.Annotations[shareNameAnnotation] = shareName

	if err := r.Client.Patch(ctx, pvc, patch); err != nil {
		return fmt.Errorf("patch pvc annotations: %w", err)
	}
	return nil
}

func (r *PVCReconciler) ensureFinalizer(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
	for _, finalizer := range pvc.Finalizers {
		if finalizer == finalizerName {
			return nil
		}
	}

	patch := client.MergeFrom(pvc.DeepCopy())
	pvc.Finalizers = append(pvc.Finalizers, finalizerName)
	if err := r.Client.Patch(ctx, pvc, patch); err != nil {
		return fmt.Errorf("patch pvc finalizer: %w", err)
	}
	return nil
}

func (r *PVCReconciler) handleDeletion(ctx context.Context, logger logr.Logger, pvc *corev1.PersistentVolumeClaim) (reconcile.Result, error) {
	managed := k8s.IsManagedPVC(pvc)
	if !managed {
		return r.removeFinalizer(ctx, pvc)
	}

	shareName := ""
	if pvc.Annotations != nil {
		shareName = pvc.Annotations[shareNameAnnotation]
	}
	if shareName == "" {
		shareName, _ = naming.ComputeShareName(pvc.Namespace, pvc.Name, "")
	}

	logger = logger.WithValues("share", shareName)
	logger.Info("cleanup started")
	r.Recorder.Event(pvc, corev1.EventTypeNormal, eventCleanupStarted, "Cleanup started for Azure File share")

	if shouldRetainShare(pvc) {
		r.Recorder.Event(pvc, corev1.EventTypeNormal, eventShareRetained, "Azure File share retained")
	} else if r.Shares != nil && shareName != "" {
		if err := r.Shares.DeleteShare(ctx, shareName); err != nil {
			r.Recorder.Event(pvc, corev1.EventTypeWarning, eventShareError, "Failed to delete Azure File share")
			return reconcile.Result{}, fmt.Errorf("delete share: %w", err)
		}
		r.Recorder.Event(pvc, corev1.EventTypeNormal, eventShareDeleted, "Azure File share deleted")
	}

	if err := r.deletePV(ctx, pvc); err != nil {
		return reconcile.Result{}, fmt.Errorf("delete pv: %w", err)
	}

	r.Recorder.Event(pvc, corev1.EventTypeNormal, eventCleanupComplete, "Cleanup complete")
	return r.removeFinalizer(ctx, pvc)
}

func (r *PVCReconciler) deletePV(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
	if pvc == nil {
		return nil
	}
	shareName := ""
	if pvc.Annotations != nil {
		shareName = pvc.Annotations[shareNameAnnotation]
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
		if f != finalizerName {
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
	return pvc.Annotations[retainShareAnnotation] == "true"
}

func (r *PVCReconciler) terminalError(logger logr.Logger, pvc *corev1.PersistentVolumeClaim, reason string, err error) (reconcile.Result, error) {
	logger.WithValues("reason", "terminal").Error(err, "terminal error")
	if pvc != nil {
		r.Recorder.Event(pvc, corev1.EventTypeWarning, reason, err.Error())
	}
	return reconcile.Result{}, nil
}

func quotaGiBFromPVC(pvc *corev1.PersistentVolumeClaim) (int32, error) {
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
