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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const defaultGeneratedSecretSuffix = "-credentials"

type secretReconciler interface {
	Reconcile(
		ctx context.Context,
		regResult domain.RegistrationResult,
		authRegistration *authregistrationv1.AuthRegistration,
		secretName string,
		controllerManagedSecret bool,
	) error
}

type statusPatcher interface {
	PatchResolvedSecretRef(ctx context.Context, authRegistration *authregistrationv1.AuthRegistration, resolvedSecretRef string) error
	PatchInvalidSpec(ctx context.Context, authRegistration *authregistrationv1.AuthRegistration, invalidSpecErr error) error
	PatchSecretReconcileFailed(ctx context.Context, authRegistration *authregistrationv1.AuthRegistration, resolvedSecretRef string, secretErr error) error
	PatchCredentialsPublished(ctx context.Context, authRegistration *authregistrationv1.AuthRegistration) error
	PatchRegistrationFailed(ctx context.Context, authRegistration *authregistrationv1.AuthRegistration, registrationErr error) error
	PatchRegistrationSucceeded(ctx context.Context, authRegistration *authregistrationv1.AuthRegistration) error
}

// ServiceRegistrationBackend abstracts the target system that stores
// AuthRegistration entries. It allows swapping CAS for another backend later.
type serviceRegistrationBackend interface {
	Upsert(ctx context.Context, registration domain.Registration) (domain.RegistrationResult, error)
	Delete(ctx context.Context, registration domain.Registration) error
}

const authRegistrationFinalizer = "k8s.cloudogu.com/auth-registration-finalizer"

// AuthRegistrationReconciler reconciles a AuthRegistration object
type AuthRegistrationReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	credentialsSecretReconciler secretReconciler
	statusPatcher               statusPatcher
	serviceRegistrationBackend  serviceRegistrationBackend
}

func NewAuthRegistrationReconciler(rtClient client.Client, scheme *runtime.Scheme, backend serviceRegistrationBackend) *AuthRegistrationReconciler {
	return &AuthRegistrationReconciler{
		Client: rtClient,
		Scheme: scheme,
		credentialsSecretReconciler: &authRegistrationSecretReconciler{
			Client: rtClient,
			Scheme: scheme,
		},
		statusPatcher: &authRegistrationStatusPatcher{
			Client: rtClient,
		},
		serviceRegistrationBackend: backend,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *AuthRegistrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&authregistrationv1.AuthRegistration{}).
		Named("authregistration").
		Complete(r)
}

// +kubebuilder:rbac:groups=k8s.cloudogu.com,resources=authregistrations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.cloudogu.com,resources=authregistrations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8s.cloudogu.com,resources=authregistrations/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *AuthRegistrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx).WithValues("authRegistration", req.NamespacedName)

	var authRegistration authregistrationv1.AuthRegistration
	if err := r.Get(ctx, req.NamespacedName, &authRegistration); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	if !authRegistration.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, &authRegistration, logger)
	}

	if controllerutil.AddFinalizer(&authRegistration, authRegistrationFinalizer) {
		if err := r.Update(ctx, &authRegistration); err != nil {
			return reconcile.Result{}, err
		}
	}

	if err := r.handleReconcile(ctx, &authRegistration, logger); err != nil {
		return reconcile.Result{}, err
	}

	logger.Info("AuthRegistration reconciled successfully")
	return reconcile.Result{}, nil
}

func (r *AuthRegistrationReconciler) handleReconcile(ctx context.Context, authRegistration *authregistrationv1.AuthRegistration, logger logr.Logger) error {
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

func (r *AuthRegistrationReconciler) cleanupObsoleteGeneratedSecret(ctx context.Context, authRegistration *authregistrationv1.AuthRegistration, previouslyResolvedSecretName string, resolvedSecretName string) error {
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
	if secret.Annotations == nil || secret.Annotations[generatedSecretAnnotationKey] != "true" {
		return false
	}

	return metav1.IsControlledBy(secret, authRegistration)
}

func (r *AuthRegistrationReconciler) handleDeletion(ctx context.Context, authRegistration *authregistrationv1.AuthRegistration, logger logr.Logger) (ctrl.Result, error) {
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

	logger.V(1).Info("AuthRegistration was deleted and backend registration removed")
	return reconcile.Result{}, nil
}

func resolveSecretName(authRegistration *authregistrationv1.AuthRegistration) (string, bool, error) {
	if authRegistration.Spec.SecretRef == nil {
		return authRegistration.Name + defaultGeneratedSecretSuffix, true, nil
	}

	secretName := strings.TrimSpace(*authRegistration.Spec.SecretRef)
	if secretName == "" {
		return "", false, fmt.Errorf("spec.secretRef must not be empty")
	}

	return secretName, false, nil
}
