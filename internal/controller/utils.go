package controller

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"aks-azureFiles-controller/internal/constants"
)

var ErrPVMismatch = errors.New("pv spec mismatch")
var ErrInvalidPVCRequest = errors.New("invalid pvc request")

func (r *PVCReconciler) ensureFinalizer(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
	for _, finalizer := range pvc.Finalizers {
		if finalizer == constants.FinalizerName {
			return nil
		}
	}

	patch := client.MergeFrom(pvc.DeepCopy())
	pvc.Finalizers = append(pvc.Finalizers, constants.FinalizerName)
	if err := r.Client.Patch(ctx, pvc, patch); err != nil {
		return fmt.Errorf("patch pvc finalizer: %w", err)
	}
	return nil
}

func (r *PVCReconciler) terminalError(logger logr.Logger, pvc *corev1.PersistentVolumeClaim, reason string, err error) (reconcile.Result, error) {
	logger.WithValues("reason", "terminal").Error(err, "terminal error")
	if pvc != nil {
		r.Recorder.Event(pvc, corev1.EventTypeWarning, reason, err.Error())
	}
	return reconcile.Result{}, nil
}

// pvMatches checks if the existing PersistentVolume matches the expectation for this PVC.
// It verifies:
// - ClaimRef matches the PVC (UID, Name, Namespace).
// - CSI Driver is correct.
// - Share Name in VolumeAttributes matches the expected share name.
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
