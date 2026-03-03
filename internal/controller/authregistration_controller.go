package controller

import (
	"context"
	"fmt"
	"strings"

	authregistrationv1 "github.com/cloudogu/k8s-auth-registration-lib/api/v1"
	"github.com/cloudogu/k8s-auth-registration-operator/internal/domain"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const defaultGeneratedSecretSuffix = "-credentials"

const authRegistrationFinalizer = "k8s.cloudogu.com/auth-registration-finalizer"
const authRegistrationSecretRefField = "k8s.cloudogu.com/auth-registration-secret-ref"

type secretReconciler interface {
	Reconcile(ctx context.Context, regResult domain.RegistrationResult, authRegistration *authregistrationv1.AuthRegistration, secretName string, controllerManagedSecret bool) error
}

type statusPatcher interface {
	PatchResolvedSecretRef(ctx context.Context, authRegistration *authregistrationv1.AuthRegistration, resolvedSecretRef string) error
	PatchInvalidSpec(ctx context.Context, authRegistration *authregistrationv1.AuthRegistration, invalidSpecErr error) error
	PatchSecretReconcileFailed(ctx context.Context, authRegistration *authregistrationv1.AuthRegistration, resolvedSecretRef string, secretErr error) error
	PatchCredentialsPublished(ctx context.Context, authRegistration *authregistrationv1.AuthRegistration) error
	PatchRegistrationFailed(ctx context.Context, authRegistration *authregistrationv1.AuthRegistration, registrationErr error) error
	PatchRegistrationSucceeded(ctx context.Context, authRegistration *authregistrationv1.AuthRegistration) error
}

type serviceRegistrationBackend interface {
	Upsert(ctx context.Context, registration domain.Registration) (domain.RegistrationResult, error)
	Delete(ctx context.Context, registration domain.Registration) error
}

type authRegistrationReconciler interface {
	handleReconcile(ctx context.Context, authRegistration *authregistrationv1.AuthRegistration, logger logr.Logger) error
	handleDeletion(ctx context.Context, authRegistration *authregistrationv1.AuthRegistration, logger logr.Logger) (ctrl.Result, error)
}

// AuthRegistrationController reacts to AuthRegistration and Secret events
// and delegates domain reconciliation to reconciler.
type AuthRegistrationController struct {
	client.Client
	reconciler authRegistrationReconciler
}

func NewAuthRegistrationController(rtClient client.Client, scheme *runtime.Scheme, backend serviceRegistrationBackend) *AuthRegistrationController {
	return &AuthRegistrationController{
		Client: rtClient,
		reconciler: newAuthRegistrationReconciler(
			rtClient,
			&authRegistrationSecretReconciler{Client: rtClient, Scheme: scheme},
			&authRegistrationStatusPatcher{Client: rtClient},
			backend,
		),
	}
}

// +kubebuilder:rbac:groups=k8s.cloudogu.com,resources=authregistrations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.cloudogu.com,resources=authregistrations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8s.cloudogu.com,resources=authregistrations/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (c *AuthRegistrationController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx).WithValues("authRegistration", req.NamespacedName)

	var authRegistration authregistrationv1.AuthRegistration
	if err := c.Get(ctx, req.NamespacedName, &authRegistration); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	if !authRegistration.DeletionTimestamp.IsZero() {
		return c.reconciler.handleDeletion(ctx, &authRegistration, logger)
	}

	if controllerutil.AddFinalizer(&authRegistration, authRegistrationFinalizer) {
		if err := c.Update(ctx, &authRegistration); err != nil {
			return reconcile.Result{}, err
		}
	}

	if err := c.reconciler.handleReconcile(ctx, &authRegistration, logger); err != nil {
		return reconcile.Result{}, err
	}

	logger.Info("AuthRegistration reconciled successfully")
	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (c *AuthRegistrationController) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&authregistrationv1.AuthRegistration{},
		authRegistrationSecretRefField,
		indexAuthRegistrationBySecretName,
	); err != nil {
		return fmt.Errorf("failed to index AuthRegistration by secret reference: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&authregistrationv1.AuthRegistration{}).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(c.mapSecretToAuthRegistrations)).
		Named("authregistration").
		Complete(c)
}

func indexAuthRegistrationBySecretName(object client.Object) []string {
	authRegistration, ok := object.(*authregistrationv1.AuthRegistration)
	if !ok {
		return nil
	}

	secretName, _, err := resolveSecretName(authRegistration)
	if err != nil {
		return nil
	}

	return []string{secretName}
}

func (c *AuthRegistrationController) mapSecretToAuthRegistrations(ctx context.Context, object client.Object) []reconcile.Request {
	secret, ok := object.(*corev1.Secret)
	if !ok {
		return nil
	}

	var authRegistrationList authregistrationv1.AuthRegistrationList
	err := c.List(ctx, &authRegistrationList, client.InNamespace(secret.Namespace), client.MatchingFields{authRegistrationSecretRefField: secret.Name})
	if err != nil {
		logf.FromContext(ctx).Error(err, "failed to map secret event to AuthRegistration resources", "namespace", secret.Namespace, "name", secret.Name)
		return nil
	}

	requests := make([]reconcile.Request, 0, len(authRegistrationList.Items))
	for _, authRegistration := range authRegistrationList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: authRegistration.Namespace,
				Name:      authRegistration.Name,
			},
		})
	}

	return requests
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
