package controller

import (
	"context"
	"fmt"
	"strings"

	authregistrationv1 "github.com/cloudogu/k8s-auth-registration-lib/api/v1"
	"github.com/cloudogu/k8s-auth-registration-operator/internal/domain"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type defaultAuthRegistrationReconciler struct {
	client.Client
	credentialsSecretReconciler secretReconciler
	statusPatcher               statusPatcher
	serviceRegistrationBackend  serviceRegistrationBackend
}

func newAuthRegistrationReconciler(rtClient client.Client, credentialsSecretReconciler secretReconciler, statusPatcher statusPatcher, backend serviceRegistrationBackend) *defaultAuthRegistrationReconciler {
	return &defaultAuthRegistrationReconciler{
		Client:                      rtClient,
		credentialsSecretReconciler: credentialsSecretReconciler,
		statusPatcher:               statusPatcher,
		serviceRegistrationBackend:  backend,
	}
}

func (r *defaultAuthRegistrationReconciler) handleReconcile(ctx context.Context, authRegistration *authregistrationv1.AuthRegistration, logger logr.Logger) error {
	regResult, err := r.serviceRegistrationBackend.Upsert(ctx, domain.FromAuthRegistration(authRegistration))
	if err != nil {
		if err := r.statusPatcher.PatchRegistrationFailed(ctx, authRegistration, err); err != nil {
			logger.Error(err, "Failed to patch status for service registration error")
		}

		return fmt.Errorf("failed to upsert service-registration: %w", err)
	}

	resolvedSecretName, isControllerManagedSecret, err := resolveSecretName(authRegistration)
	if err != nil {
		if err := r.statusPatcher.PatchInvalidSpec(ctx, authRegistration, err); err != nil {
			logger.Error(err, "Failed to patch status for invalid spec")
		}

		return fmt.Errorf("failed to resolve secret reference: %w", err)
	}

	previouslyResolvedSecretName := authRegistration.Status.ResolvedSecretRef

	if err := r.statusPatcher.PatchResolvedSecretRef(ctx, authRegistration, resolvedSecretName); err != nil {
		return fmt.Errorf("failed to patch status.resolvedSecretName: %w", err)
	}

	if err := r.credentialsSecretReconciler.Reconcile(ctx, regResult, authRegistration, resolvedSecretName, isControllerManagedSecret); err != nil {
		if err := r.statusPatcher.PatchSecretReconcileFailed(ctx, authRegistration, resolvedSecretName, err); err != nil {
			logger.Error(err, "Failed to patch status for Secret reconcile error")
		}

		return fmt.Errorf("failed to reconcile secret: %w", err)
	}

	if err := r.cleanupObsoleteGeneratedSecret(ctx, authRegistration, previouslyResolvedSecretName, resolvedSecretName); err != nil {
		return fmt.Errorf("failed to cleanup obsolete generated secret: %w", err)
	}

	if err := r.statusPatcher.PatchCredentialsPublished(ctx, authRegistration); err != nil {
		return fmt.Errorf("failed to patch condition: %w", err)
	}

	if err := r.statusPatcher.PatchRegistrationSucceeded(ctx, authRegistration); err != nil {
		return fmt.Errorf("failed to patch condition: %w", err)
	}

	return nil
}

func (r *defaultAuthRegistrationReconciler) cleanupObsoleteGeneratedSecret(ctx context.Context, authRegistration *authregistrationv1.AuthRegistration, previouslyResolvedSecretName string, resolvedSecretName string) error {
	previouslyResolvedSecretName = strings.TrimSpace(previouslyResolvedSecretName)
	if previouslyResolvedSecretName == "" || previouslyResolvedSecretName == resolvedSecretName {
		return nil
	}

	previousSecretKey := types.NamespacedName{
		Name:      previouslyResolvedSecretName,
		Namespace: authRegistration.Namespace,
	}

	var previousSecret corev1.Secret
	if err := r.Get(ctx, previousSecretKey, &previousSecret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("failed to get previous secret %q: %w", previouslyResolvedSecretName, err)
	}

	if !isControllerGeneratedSecret(&previousSecret, authRegistration) {
		return nil
	}

	if err := r.Delete(ctx, &previousSecret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("failed to delete previous secret %q: %w", previouslyResolvedSecretName, err)
	}

	return nil
}

func isControllerGeneratedSecret(secret *corev1.Secret, authRegistration *authregistrationv1.AuthRegistration) bool {
	return metav1.IsControlledBy(secret, authRegistration)
}

func (r *defaultAuthRegistrationReconciler) handleDeletion(ctx context.Context, authRegistration *authregistrationv1.AuthRegistration, logger logr.Logger) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(authRegistration, authRegistrationFinalizer) {
		return reconcile.Result{}, nil
	}

	if err := r.serviceRegistrationBackend.Delete(ctx, domain.FromAuthRegistration(authRegistration)); err != nil {
		logger.Error(err, "Failed to remove service registration from backend")
		return reconcile.Result{}, err
	}

	// the credentials secret will be removed automatically, because an owener-reference is set

	controllerutil.RemoveFinalizer(authRegistration, authRegistrationFinalizer)
	if err := r.Update(ctx, authRegistration); err != nil {
		return reconcile.Result{}, err
	}

	logger.Info("AuthRegistration was deleted and backend registration removed")
	return reconcile.Result{}, nil
}
