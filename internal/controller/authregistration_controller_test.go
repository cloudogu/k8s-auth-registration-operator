package controller

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	authregistrationv1 "github.com/cloudogu/k8s-auth-registration-lib/api/v1"
	"github.com/cloudogu/k8s-auth-registration-operator/internal/domain"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestResolveSecretName(t *testing.T) {
	t.Run("returns generated controller-managed secret name when spec.secretRef is nil", func(t *testing.T) {
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")

		secretName, isControllerManaged, err := resolveSecretName(authRegistration)

		require.NoError(t, err)
		assert.Equal(t, "auth-reg"+defaultGeneratedSecretSuffix, secretName)
		assert.True(t, isControllerManaged)
	})

	t.Run("returns trimmed unmanaged secret name when spec.secretRef is set", func(t *testing.T) {
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		authRegistration.Spec.SecretRef = stringPtrForControllerTest("  custom-secret  ")

		secretName, isControllerManaged, err := resolveSecretName(authRegistration)

		require.NoError(t, err)
		assert.Equal(t, "custom-secret", secretName)
		assert.False(t, isControllerManaged)
	})

	t.Run("returns an error when spec.secretRef is empty after trimming", func(t *testing.T) {
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		authRegistration.Spec.SecretRef = stringPtrForControllerTest(" \n\t ")

		secretName, isControllerManaged, err := resolveSecretName(authRegistration)

		require.Error(t, err)
		assert.ErrorContains(t, err, "spec.secretRef must not be empty")
		assert.Empty(t, secretName)
		assert.False(t, isControllerManaged)
	})
}

func TestNewAuthRegistrationReconciler(t *testing.T) {
	t.Run("constructs reconciler with default collaborators and provided backend", func(t *testing.T) {
		scheme := newAuthRegistrationControllerSchemeForTest(t)
		client := fake.NewClientBuilder().WithScheme(scheme).Build()
		mockServiceRegistrationBackend := newMockServiceRegistrationBackend(t)

		reconciler := NewAuthRegistrationReconciler(client, scheme, mockServiceRegistrationBackend)

		require.NotNil(t, reconciler)
		assert.Same(t, client, reconciler.Client)
		assert.Same(t, scheme, reconciler.Scheme)
		assert.Same(t, mockServiceRegistrationBackend, reconciler.serviceRegistrationBackend)
		_, hasDefaultSecretReconciler := reconciler.credentialsSecretReconciler.(*authRegistrationSecretReconciler)
		_, hasDefaultStatusPatcher := reconciler.statusPatcher.(*authRegistrationStatusPatcher)
		assert.True(t, hasDefaultSecretReconciler)
		assert.True(t, hasDefaultStatusPatcher)
	})
}

func TestAuthRegistrationReconciler_HandleReconcile(t *testing.T) {
	t.Run("reconciles successfully with generated controller-managed secret", func(t *testing.T) {
		mockServiceRegistrationBackend := newMockServiceRegistrationBackend(t)
		mockStatusPatcher := newMockStatusPatcher(t)
		mockSecretReconciler := newMockSecretReconciler(t)
		reconciler, _ := newAuthRegistrationControllerReconcilerForTest(t, nil, mockServiceRegistrationBackend, mockStatusPatcher, mockSecretReconciler)
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		registrationResult := newOIDCRegistrationResultForControllerTest()

		mockServiceRegistrationBackend.EXPECT().
			Upsert(mock.Anything, matchRegistration(domain.FromAuthRegistration(authRegistration))).
			Return(registrationResult, nil).
			Once()
		mockStatusPatcher.EXPECT().
			PatchResolvedSecretRef(mock.Anything, authRegistration, "auth-reg"+defaultGeneratedSecretSuffix).
			Return(nil).
			Once()
		mockSecretReconciler.EXPECT().
			Reconcile(mock.Anything, registrationResult, authRegistration, "auth-reg"+defaultGeneratedSecretSuffix, true).
			Return(nil).
			Once()
		mockStatusPatcher.EXPECT().
			PatchCredentialsPublished(mock.Anything, authRegistration).
			Return(nil).
			Once()
		mockStatusPatcher.EXPECT().
			PatchRegistrationSucceeded(mock.Anything, authRegistration).
			Return(nil).
			Once()

		err := reconciler.handleReconcile(context.Background(), authRegistration, logr.Discard())

		require.NoError(t, err)
	})

	t.Run("reconciles successfully with explicit unmanaged secret ref", func(t *testing.T) {
		mockServiceRegistrationBackend := newMockServiceRegistrationBackend(t)
		mockStatusPatcher := newMockStatusPatcher(t)
		mockSecretReconciler := newMockSecretReconciler(t)
		reconciler, _ := newAuthRegistrationControllerReconcilerForTest(t, nil, mockServiceRegistrationBackend, mockStatusPatcher, mockSecretReconciler)
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		authRegistration.Spec.SecretRef = stringPtrForControllerTest("  given-secret  ")
		registrationResult := newOIDCRegistrationResultForControllerTest()

		mockServiceRegistrationBackend.EXPECT().
			Upsert(mock.Anything, matchRegistration(domain.FromAuthRegistration(authRegistration))).
			Return(registrationResult, nil).
			Once()
		mockStatusPatcher.EXPECT().
			PatchResolvedSecretRef(mock.Anything, authRegistration, "given-secret").
			Return(nil).
			Once()
		mockSecretReconciler.EXPECT().
			Reconcile(mock.Anything, registrationResult, authRegistration, "given-secret", false).
			Return(nil).
			Once()
		mockStatusPatcher.EXPECT().
			PatchCredentialsPublished(mock.Anything, authRegistration).
			Return(nil).
			Once()
		mockStatusPatcher.EXPECT().
			PatchRegistrationSucceeded(mock.Anything, authRegistration).
			Return(nil).
			Once()

		err := reconciler.handleReconcile(context.Background(), authRegistration, logr.Discard())

		require.NoError(t, err)
	})

	t.Run("deletes previously resolved generated secret when switching to explicit secretRef", func(t *testing.T) {
		mockServiceRegistrationBackend := newMockServiceRegistrationBackend(t)
		mockStatusPatcher := newMockStatusPatcher(t)
		mockSecretReconciler := newMockSecretReconciler(t)
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		authRegistration.Status.ResolvedSecretRef = "auth-reg-credentials"
		authRegistration.Spec.SecretRef = stringPtrForControllerTest("custom-secret")
		previousGeneratedSecret := newGeneratedSecretForControllerTest(authRegistration, "auth-reg-credentials")
		key := types.NamespacedName{Name: previousGeneratedSecret.Name, Namespace: previousGeneratedSecret.Namespace}
		reconciler, c := newAuthRegistrationControllerReconcilerForTest(
			t,
			nil,
			mockServiceRegistrationBackend,
			mockStatusPatcher,
			mockSecretReconciler,
			previousGeneratedSecret,
		)
		registrationResult := newOIDCRegistrationResultForControllerTest()

		mockServiceRegistrationBackend.EXPECT().
			Upsert(mock.Anything, matchRegistration(domain.FromAuthRegistration(authRegistration))).
			Return(registrationResult, nil).
			Once()
		mockStatusPatcher.EXPECT().
			PatchResolvedSecretRef(mock.Anything, authRegistration, "custom-secret").
			Return(nil).
			Once()
		mockSecretReconciler.EXPECT().
			Reconcile(mock.Anything, registrationResult, authRegistration, "custom-secret", false).
			Return(nil).
			Once()
		mockStatusPatcher.EXPECT().
			PatchCredentialsPublished(mock.Anything, authRegistration).
			Return(nil).
			Once()
		mockStatusPatcher.EXPECT().
			PatchRegistrationSucceeded(mock.Anything, authRegistration).
			Return(nil).
			Once()

		err := reconciler.handleReconcile(context.Background(), authRegistration, logr.Discard())

		require.NoError(t, err)
		deletedSecret := &corev1.Secret{}
		getErr := c.Get(context.Background(), key, deletedSecret)
		assert.True(t, apierrors.IsNotFound(getErr))
	})

	t.Run("keeps previously resolved secret when it is not controller-generated", func(t *testing.T) {
		mockServiceRegistrationBackend := newMockServiceRegistrationBackend(t)
		mockStatusPatcher := newMockStatusPatcher(t)
		mockSecretReconciler := newMockSecretReconciler(t)
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		authRegistration.Status.ResolvedSecretRef = "existing-user-secret"
		authRegistration.Spec.SecretRef = stringPtrForControllerTest("custom-secret")
		previousUnmanagedSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "existing-user-secret",
				Namespace: "ecosystem",
			},
		}
		key := types.NamespacedName{Name: previousUnmanagedSecret.Name, Namespace: previousUnmanagedSecret.Namespace}
		reconciler, c := newAuthRegistrationControllerReconcilerForTest(
			t,
			nil,
			mockServiceRegistrationBackend,
			mockStatusPatcher,
			mockSecretReconciler,
			previousUnmanagedSecret,
		)
		registrationResult := newOIDCRegistrationResultForControllerTest()

		mockServiceRegistrationBackend.EXPECT().
			Upsert(mock.Anything, matchRegistration(domain.FromAuthRegistration(authRegistration))).
			Return(registrationResult, nil).
			Once()
		mockStatusPatcher.EXPECT().
			PatchResolvedSecretRef(mock.Anything, authRegistration, "custom-secret").
			Return(nil).
			Once()
		mockSecretReconciler.EXPECT().
			Reconcile(mock.Anything, registrationResult, authRegistration, "custom-secret", false).
			Return(nil).
			Once()
		mockStatusPatcher.EXPECT().
			PatchCredentialsPublished(mock.Anything, authRegistration).
			Return(nil).
			Once()
		mockStatusPatcher.EXPECT().
			PatchRegistrationSucceeded(mock.Anything, authRegistration).
			Return(nil).
			Once()

		err := reconciler.handleReconcile(context.Background(), authRegistration, logr.Discard())

		require.NoError(t, err)
		remainingSecret := &corev1.Secret{}
		require.NoError(t, c.Get(context.Background(), key, remainingSecret))
	})

	t.Run("returns wrapped error when deleting obsolete generated secret fails", func(t *testing.T) {
		mockServiceRegistrationBackend := newMockServiceRegistrationBackend(t)
		mockStatusPatcher := newMockStatusPatcher(t)
		mockSecretReconciler := newMockSecretReconciler(t)
		recorder := &authRegistrationControllerClientRecorder{
			deleteErr: errors.New("delete failed"),
		}
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		authRegistration.Status.ResolvedSecretRef = "auth-reg-credentials"
		authRegistration.Spec.SecretRef = stringPtrForControllerTest("custom-secret")
		previousGeneratedSecret := newGeneratedSecretForControllerTest(authRegistration, "auth-reg-credentials")
		reconciler, c := newAuthRegistrationControllerReconcilerForTest(
			t,
			recorder,
			mockServiceRegistrationBackend,
			mockStatusPatcher,
			mockSecretReconciler,
			previousGeneratedSecret,
		)
		registrationResult := newOIDCRegistrationResultForControllerTest()

		mockServiceRegistrationBackend.EXPECT().
			Upsert(mock.Anything, matchRegistration(domain.FromAuthRegistration(authRegistration))).
			Return(registrationResult, nil).
			Once()
		mockStatusPatcher.EXPECT().
			PatchResolvedSecretRef(mock.Anything, authRegistration, "custom-secret").
			Return(nil).
			Once()
		mockSecretReconciler.EXPECT().
			Reconcile(mock.Anything, registrationResult, authRegistration, "custom-secret", false).
			Return(nil).
			Once()

		err := reconciler.handleReconcile(context.Background(), authRegistration, logr.Discard())

		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to cleanup obsolete generated secret")
		assert.ErrorContains(t, err, "delete failed")

		remainingSecret := &corev1.Secret{}
		key := types.NamespacedName{Name: previousGeneratedSecret.Name, Namespace: previousGeneratedSecret.Namespace}
		require.NoError(t, c.Get(context.Background(), key, remainingSecret))
	})

	t.Run("returns wrapped error and patches registration failed when backend upsert fails", func(t *testing.T) {
		mockServiceRegistrationBackend := newMockServiceRegistrationBackend(t)
		mockStatusPatcher := newMockStatusPatcher(t)
		mockSecretReconciler := newMockSecretReconciler(t)
		reconciler, _ := newAuthRegistrationControllerReconcilerForTest(t, nil, mockServiceRegistrationBackend, mockStatusPatcher, mockSecretReconciler)
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		upsertErr := errors.New("backend unavailable")

		mockServiceRegistrationBackend.EXPECT().
			Upsert(mock.Anything, matchRegistration(domain.FromAuthRegistration(authRegistration))).
			Return(domain.RegistrationResult{}, upsertErr).
			Once()
		mockStatusPatcher.EXPECT().
			PatchRegistrationFailed(mock.Anything, authRegistration, upsertErr).
			Return(nil).
			Once()

		err := reconciler.handleReconcile(context.Background(), authRegistration, logr.Discard())

		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to upsert service-registration")
		assert.ErrorContains(t, err, "backend unavailable")
	})

	t.Run("returns original upsert error even when patching registration failed status also fails", func(t *testing.T) {
		mockServiceRegistrationBackend := newMockServiceRegistrationBackend(t)
		mockStatusPatcher := newMockStatusPatcher(t)
		mockSecretReconciler := newMockSecretReconciler(t)
		reconciler, _ := newAuthRegistrationControllerReconcilerForTest(t, nil, mockServiceRegistrationBackend, mockStatusPatcher, mockSecretReconciler)
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		upsertErr := errors.New("backend unavailable")
		patchErr := errors.New("status patch failed")

		mockServiceRegistrationBackend.EXPECT().
			Upsert(mock.Anything, matchRegistration(domain.FromAuthRegistration(authRegistration))).
			Return(domain.RegistrationResult{}, upsertErr).
			Once()
		mockStatusPatcher.EXPECT().
			PatchRegistrationFailed(mock.Anything, authRegistration, upsertErr).
			Return(patchErr).
			Once()

		err := reconciler.handleReconcile(context.Background(), authRegistration, logr.Discard())

		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to upsert service-registration")
		assert.ErrorContains(t, err, "backend unavailable")
		assert.NotContains(t, err.Error(), "status patch failed")
	})

	t.Run("returns wrapped error and patches invalid spec when secret reference is empty", func(t *testing.T) {
		mockServiceRegistrationBackend := newMockServiceRegistrationBackend(t)
		mockStatusPatcher := newMockStatusPatcher(t)
		mockSecretReconciler := newMockSecretReconciler(t)
		reconciler, _ := newAuthRegistrationControllerReconcilerForTest(t, nil, mockServiceRegistrationBackend, mockStatusPatcher, mockSecretReconciler)
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		authRegistration.Spec.SecretRef = stringPtrForControllerTest("   ")
		registrationResult := newOIDCRegistrationResultForControllerTest()

		mockServiceRegistrationBackend.EXPECT().
			Upsert(mock.Anything, matchRegistration(domain.FromAuthRegistration(authRegistration))).
			Return(registrationResult, nil).
			Once()
		mockStatusPatcher.EXPECT().
			PatchInvalidSpec(mock.Anything, authRegistration, mock.MatchedBy(func(err error) bool {
				return err != nil && strings.Contains(err.Error(), "spec.secretRef must not be empty")
			})).
			Return(nil).
			Once()

		err := reconciler.handleReconcile(context.Background(), authRegistration, logr.Discard())

		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to resolve secret reference")
		assert.ErrorContains(t, err, "spec.secretRef must not be empty")
	})

	t.Run("returns original resolve-secret error even when invalid-spec patching fails", func(t *testing.T) {
		mockServiceRegistrationBackend := newMockServiceRegistrationBackend(t)
		mockStatusPatcher := newMockStatusPatcher(t)
		mockSecretReconciler := newMockSecretReconciler(t)
		reconciler, _ := newAuthRegistrationControllerReconcilerForTest(t, nil, mockServiceRegistrationBackend, mockStatusPatcher, mockSecretReconciler)
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		authRegistration.Spec.SecretRef = stringPtrForControllerTest("  ")
		registrationResult := newOIDCRegistrationResultForControllerTest()

		mockServiceRegistrationBackend.EXPECT().
			Upsert(mock.Anything, matchRegistration(domain.FromAuthRegistration(authRegistration))).
			Return(registrationResult, nil).
			Once()
		mockStatusPatcher.EXPECT().
			PatchInvalidSpec(mock.Anything, authRegistration, mock.Anything).
			Return(errors.New("status patch failed")).
			Once()

		err := reconciler.handleReconcile(context.Background(), authRegistration, logr.Discard())

		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to resolve secret reference")
		assert.ErrorContains(t, err, "spec.secretRef must not be empty")
		assert.NotContains(t, err.Error(), "status patch failed")
	})

	t.Run("returns wrapped error when patching resolved secret reference fails", func(t *testing.T) {
		mockServiceRegistrationBackend := newMockServiceRegistrationBackend(t)
		mockStatusPatcher := newMockStatusPatcher(t)
		mockSecretReconciler := newMockSecretReconciler(t)
		reconciler, _ := newAuthRegistrationControllerReconcilerForTest(t, nil, mockServiceRegistrationBackend, mockStatusPatcher, mockSecretReconciler)
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		registrationResult := newOIDCRegistrationResultForControllerTest()

		mockServiceRegistrationBackend.EXPECT().
			Upsert(mock.Anything, matchRegistration(domain.FromAuthRegistration(authRegistration))).
			Return(registrationResult, nil).
			Once()
		mockStatusPatcher.EXPECT().
			PatchResolvedSecretRef(mock.Anything, authRegistration, "auth-reg"+defaultGeneratedSecretSuffix).
			Return(errors.New("status patch failed")).
			Once()

		err := reconciler.handleReconcile(context.Background(), authRegistration, logr.Discard())

		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to patch status.resolvedSecretName")
		assert.ErrorContains(t, err, "status patch failed")
	})

	t.Run("returns wrapped error and patches secret-reconcile failure when secret reconcile fails", func(t *testing.T) {
		mockServiceRegistrationBackend := newMockServiceRegistrationBackend(t)
		mockStatusPatcher := newMockStatusPatcher(t)
		mockSecretReconciler := newMockSecretReconciler(t)
		reconciler, _ := newAuthRegistrationControllerReconcilerForTest(t, nil, mockServiceRegistrationBackend, mockStatusPatcher, mockSecretReconciler)
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		registrationResult := newOIDCRegistrationResultForControllerTest()
		secretErr := errors.New("secret reconcile failed")

		mockServiceRegistrationBackend.EXPECT().
			Upsert(mock.Anything, matchRegistration(domain.FromAuthRegistration(authRegistration))).
			Return(registrationResult, nil).
			Once()
		mockStatusPatcher.EXPECT().
			PatchResolvedSecretRef(mock.Anything, authRegistration, "auth-reg"+defaultGeneratedSecretSuffix).
			Return(nil).
			Once()
		mockSecretReconciler.EXPECT().
			Reconcile(mock.Anything, registrationResult, authRegistration, "auth-reg"+defaultGeneratedSecretSuffix, true).
			Return(secretErr).
			Once()
		mockStatusPatcher.EXPECT().
			PatchSecretReconcileFailed(
				mock.Anything,
				authRegistration,
				"auth-reg"+defaultGeneratedSecretSuffix,
				secretErr,
			).
			Return(nil).
			Once()

		err := reconciler.handleReconcile(context.Background(), authRegistration, logr.Discard())

		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to reconcile secret")
		assert.ErrorContains(t, err, "secret reconcile failed")
	})

	t.Run("returns original secret reconcile error even when secret failure status patch fails", func(t *testing.T) {
		mockServiceRegistrationBackend := newMockServiceRegistrationBackend(t)
		mockStatusPatcher := newMockStatusPatcher(t)
		mockSecretReconciler := newMockSecretReconciler(t)
		reconciler, _ := newAuthRegistrationControllerReconcilerForTest(t, nil, mockServiceRegistrationBackend, mockStatusPatcher, mockSecretReconciler)
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		registrationResult := newOIDCRegistrationResultForControllerTest()
		secretErr := errors.New("secret reconcile failed")

		mockServiceRegistrationBackend.EXPECT().
			Upsert(mock.Anything, matchRegistration(domain.FromAuthRegistration(authRegistration))).
			Return(registrationResult, nil).
			Once()
		mockStatusPatcher.EXPECT().
			PatchResolvedSecretRef(mock.Anything, authRegistration, "auth-reg"+defaultGeneratedSecretSuffix).
			Return(nil).
			Once()
		mockSecretReconciler.EXPECT().
			Reconcile(mock.Anything, registrationResult, authRegistration, "auth-reg"+defaultGeneratedSecretSuffix, true).
			Return(secretErr).
			Once()
		mockStatusPatcher.EXPECT().
			PatchSecretReconcileFailed(
				mock.Anything,
				authRegistration,
				"auth-reg"+defaultGeneratedSecretSuffix,
				secretErr,
			).
			Return(errors.New("status patch failed")).
			Once()

		err := reconciler.handleReconcile(context.Background(), authRegistration, logr.Discard())

		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to reconcile secret")
		assert.ErrorContains(t, err, "secret reconcile failed")
		assert.NotContains(t, err.Error(), "status patch failed")
	})

	t.Run("returns wrapped error when patching credentials-published condition fails", func(t *testing.T) {
		mockServiceRegistrationBackend := newMockServiceRegistrationBackend(t)
		mockStatusPatcher := newMockStatusPatcher(t)
		mockSecretReconciler := newMockSecretReconciler(t)
		reconciler, _ := newAuthRegistrationControllerReconcilerForTest(t, nil, mockServiceRegistrationBackend, mockStatusPatcher, mockSecretReconciler)
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		registrationResult := newOIDCRegistrationResultForControllerTest()

		mockServiceRegistrationBackend.EXPECT().
			Upsert(mock.Anything, matchRegistration(domain.FromAuthRegistration(authRegistration))).
			Return(registrationResult, nil).
			Once()
		mockStatusPatcher.EXPECT().
			PatchResolvedSecretRef(mock.Anything, authRegistration, "auth-reg"+defaultGeneratedSecretSuffix).
			Return(nil).
			Once()
		mockSecretReconciler.EXPECT().
			Reconcile(mock.Anything, registrationResult, authRegistration, "auth-reg"+defaultGeneratedSecretSuffix, true).
			Return(nil).
			Once()
		mockStatusPatcher.EXPECT().
			PatchCredentialsPublished(mock.Anything, authRegistration).
			Return(errors.New("status patch failed")).
			Once()

		err := reconciler.handleReconcile(context.Background(), authRegistration, logr.Discard())

		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to patch condition")
		assert.ErrorContains(t, err, "status patch failed")
	})

	t.Run("returns wrapped error when patching registration-succeeded condition fails", func(t *testing.T) {
		mockServiceRegistrationBackend := newMockServiceRegistrationBackend(t)
		mockStatusPatcher := newMockStatusPatcher(t)
		mockSecretReconciler := newMockSecretReconciler(t)
		reconciler, _ := newAuthRegistrationControllerReconcilerForTest(t, nil, mockServiceRegistrationBackend, mockStatusPatcher, mockSecretReconciler)
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		registrationResult := newOIDCRegistrationResultForControllerTest()

		mockServiceRegistrationBackend.EXPECT().
			Upsert(mock.Anything, matchRegistration(domain.FromAuthRegistration(authRegistration))).
			Return(registrationResult, nil).
			Once()
		mockStatusPatcher.EXPECT().
			PatchResolvedSecretRef(mock.Anything, authRegistration, "auth-reg"+defaultGeneratedSecretSuffix).
			Return(nil).
			Once()
		mockSecretReconciler.EXPECT().
			Reconcile(mock.Anything, registrationResult, authRegistration, "auth-reg"+defaultGeneratedSecretSuffix, true).
			Return(nil).
			Once()
		mockStatusPatcher.EXPECT().
			PatchCredentialsPublished(mock.Anything, authRegistration).
			Return(nil).
			Once()
		mockStatusPatcher.EXPECT().
			PatchRegistrationSucceeded(mock.Anything, authRegistration).
			Return(errors.New("status patch failed")).
			Once()

		err := reconciler.handleReconcile(context.Background(), authRegistration, logr.Discard())

		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to patch condition")
		assert.ErrorContains(t, err, "status patch failed")
	})
}

func TestAuthRegistrationReconciler_HandleDeletion(t *testing.T) {
	t.Run("returns without backend interaction when finalizer is missing", func(t *testing.T) {
		mockServiceRegistrationBackend := newMockServiceRegistrationBackend(t)
		mockStatusPatcher := newMockStatusPatcher(t)
		mockSecretReconciler := newMockSecretReconciler(t)
		reconciler, _ := newAuthRegistrationControllerReconcilerForTest(t, nil, mockServiceRegistrationBackend, mockStatusPatcher, mockSecretReconciler)
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")

		result, err := reconciler.handleDeletion(context.Background(), authRegistration, logr.Discard())

		require.NoError(t, err)
		assert.Equal(t, ctrl.Result{}, result)
		assert.False(t, containsFinalizer(authRegistration, authRegistrationFinalizer))
	})

	t.Run("returns backend delete error when finalizer exists", func(t *testing.T) {
		mockServiceRegistrationBackend := newMockServiceRegistrationBackend(t)
		mockStatusPatcher := newMockStatusPatcher(t)
		mockSecretReconciler := newMockSecretReconciler(t)
		reconciler, _ := newAuthRegistrationControllerReconcilerForTest(t, nil, mockServiceRegistrationBackend, mockStatusPatcher, mockSecretReconciler)
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		authRegistration.Finalizers = []string{authRegistrationFinalizer}
		deleteErr := errors.New("delete failed")

		mockServiceRegistrationBackend.EXPECT().
			Delete(mock.Anything, matchRegistration(domain.FromAuthRegistration(authRegistration))).
			Return(deleteErr).
			Once()

		result, err := reconciler.handleDeletion(context.Background(), authRegistration, logr.Discard())

		require.Error(t, err)
		assert.EqualError(t, err, "delete failed")
		assert.Equal(t, ctrl.Result{}, result)
		assert.True(t, containsFinalizer(authRegistration, authRegistrationFinalizer))
	})

	t.Run("removes finalizer and updates resource when backend delete succeeds", func(t *testing.T) {
		mockServiceRegistrationBackend := newMockServiceRegistrationBackend(t)
		mockStatusPatcher := newMockStatusPatcher(t)
		mockSecretReconciler := newMockSecretReconciler(t)
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		authRegistration.Finalizers = []string{authRegistrationFinalizer}
		key := types.NamespacedName{Name: authRegistration.Name, Namespace: authRegistration.Namespace}
		reconciler, c := newAuthRegistrationControllerReconcilerForTest(t, nil, mockServiceRegistrationBackend, mockStatusPatcher, mockSecretReconciler, authRegistration)
		stored := getAuthRegistrationFromClientForControllerTest(t, c, key)

		mockServiceRegistrationBackend.EXPECT().
			Delete(mock.Anything, matchRegistration(domain.FromAuthRegistration(stored))).
			Return(nil).
			Once()

		result, err := reconciler.handleDeletion(context.Background(), stored, logr.Discard())

		require.NoError(t, err)
		assert.Equal(t, ctrl.Result{}, result)
		assert.False(t, containsFinalizer(stored, authRegistrationFinalizer))

		updated := getAuthRegistrationFromClientForControllerTest(t, c, key)
		assert.False(t, containsFinalizer(updated, authRegistrationFinalizer))
	})

	t.Run("returns update error when finalizer removal cannot be persisted", func(t *testing.T) {
		mockServiceRegistrationBackend := newMockServiceRegistrationBackend(t)
		mockStatusPatcher := newMockStatusPatcher(t)
		mockSecretReconciler := newMockSecretReconciler(t)
		recorder := &authRegistrationControllerClientRecorder{
			updateErr: errors.New("update failed"),
		}
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		authRegistration.Finalizers = []string{authRegistrationFinalizer}
		key := types.NamespacedName{Name: authRegistration.Name, Namespace: authRegistration.Namespace}
		reconciler, c := newAuthRegistrationControllerReconcilerForTest(
			t,
			recorder,
			mockServiceRegistrationBackend,
			mockStatusPatcher,
			mockSecretReconciler,
			authRegistration,
		)
		stored := getAuthRegistrationFromClientForControllerTest(t, c, key)

		mockServiceRegistrationBackend.EXPECT().
			Delete(mock.Anything, matchRegistration(domain.FromAuthRegistration(stored))).
			Return(nil).
			Once()

		result, err := reconciler.handleDeletion(context.Background(), stored, logr.Discard())

		require.Error(t, err)
		assert.EqualError(t, err, "update failed")
		assert.Equal(t, ctrl.Result{}, result)

		updated := getAuthRegistrationFromClientForControllerTest(t, c, key)
		assert.True(t, containsFinalizer(updated, authRegistrationFinalizer))
	})
}

func TestAuthRegistrationReconciler_Reconcile(t *testing.T) {
	t.Run("returns nil when resource is not found", func(t *testing.T) {
		mockServiceRegistrationBackend := newMockServiceRegistrationBackend(t)
		mockStatusPatcher := newMockStatusPatcher(t)
		mockSecretReconciler := newMockSecretReconciler(t)
		reconciler, _ := newAuthRegistrationControllerReconcilerForTest(t, nil, mockServiceRegistrationBackend, mockStatusPatcher, mockSecretReconciler)
		request := ctrl.Request{NamespacedName: types.NamespacedName{Name: "auth-reg", Namespace: "ecosystem"}}

		result, err := reconciler.Reconcile(context.Background(), request)

		require.NoError(t, err)
		assert.Equal(t, ctrl.Result{}, result)
	})

	t.Run("returns non-notfound get errors", func(t *testing.T) {
		mockServiceRegistrationBackend := newMockServiceRegistrationBackend(t)
		mockStatusPatcher := newMockStatusPatcher(t)
		mockSecretReconciler := newMockSecretReconciler(t)
		recorder := &authRegistrationControllerClientRecorder{
			getErr: errors.New("get failed"),
		}
		reconciler, _ := newAuthRegistrationControllerReconcilerForTest(t, recorder, mockServiceRegistrationBackend, mockStatusPatcher, mockSecretReconciler)
		request := ctrl.Request{NamespacedName: types.NamespacedName{Name: "auth-reg", Namespace: "ecosystem"}}

		result, err := reconciler.Reconcile(context.Background(), request)

		require.Error(t, err)
		assert.EqualError(t, err, "get failed")
		assert.Equal(t, ctrl.Result{}, result)
	})

	t.Run("returns update error when adding finalizer fails", func(t *testing.T) {
		mockServiceRegistrationBackend := newMockServiceRegistrationBackend(t)
		mockStatusPatcher := newMockStatusPatcher(t)
		mockSecretReconciler := newMockSecretReconciler(t)
		recorder := &authRegistrationControllerClientRecorder{
			updateErr: errors.New("update failed"),
		}
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		reconciler, _ := newAuthRegistrationControllerReconcilerForTest(
			t,
			recorder,
			mockServiceRegistrationBackend,
			mockStatusPatcher,
			mockSecretReconciler,
			authRegistration,
		)
		request := ctrl.Request{NamespacedName: types.NamespacedName{Name: "auth-reg", Namespace: "ecosystem"}}

		result, err := reconciler.Reconcile(context.Background(), request)

		require.Error(t, err)
		assert.EqualError(t, err, "update failed")
		assert.Equal(t, ctrl.Result{}, result)
	})

	t.Run("uses deletion flow when deletion timestamp is set", func(t *testing.T) {
		mockServiceRegistrationBackend := newMockServiceRegistrationBackend(t)
		mockStatusPatcher := newMockStatusPatcher(t)
		mockSecretReconciler := newMockSecretReconciler(t)
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		authRegistration.Finalizers = []string{authRegistrationFinalizer}
		deletionTime := metav1.NewTime(time.Now())
		authRegistration.DeletionTimestamp = &deletionTime
		key := types.NamespacedName{Name: authRegistration.Name, Namespace: authRegistration.Namespace}
		reconciler, c := newAuthRegistrationControllerReconcilerForTest(
			t,
			nil,
			mockServiceRegistrationBackend,
			mockStatusPatcher,
			mockSecretReconciler,
			authRegistration,
		)

		mockServiceRegistrationBackend.EXPECT().
			Delete(mock.Anything, matchRegistration(domain.FromAuthRegistration(authRegistration))).
			Return(nil).
			Once()

		result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: key})

		require.NoError(t, err)
		assert.Equal(t, ctrl.Result{}, result)
		updated := &authregistrationv1.AuthRegistration{}
		getErr := c.Get(context.Background(), key, updated)
		if apierrors.IsNotFound(getErr) {
			return
		}

		require.NoError(t, getErr)
		assert.False(t, containsFinalizer(updated, authRegistrationFinalizer))
	})

	t.Run("persists finalizer before returning handleReconcile errors", func(t *testing.T) {
		mockServiceRegistrationBackend := newMockServiceRegistrationBackend(t)
		mockStatusPatcher := newMockStatusPatcher(t)
		mockSecretReconciler := newMockSecretReconciler(t)
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		key := types.NamespacedName{Name: authRegistration.Name, Namespace: authRegistration.Namespace}
		reconciler, c := newAuthRegistrationControllerReconcilerForTest(
			t,
			nil,
			mockServiceRegistrationBackend,
			mockStatusPatcher,
			mockSecretReconciler,
			authRegistration,
		)
		upsertErr := errors.New("backend unavailable")

		mockServiceRegistrationBackend.EXPECT().
			Upsert(mock.Anything, matchRegistration(domain.FromAuthRegistration(authRegistration))).
			Return(domain.RegistrationResult{}, upsertErr).
			Once()
		mockStatusPatcher.EXPECT().
			PatchRegistrationFailed(mock.Anything, matchAuthRegistration("ecosystem", "auth-reg"), upsertErr).
			Return(nil).
			Once()

		result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: key})

		require.Error(t, err)
		assert.Equal(t, ctrl.Result{}, result)
		assert.ErrorContains(t, err, "failed to upsert service-registration")

		updated := getAuthRegistrationFromClientForControllerTest(t, c, key)
		assert.True(t, containsFinalizer(updated, authRegistrationFinalizer))
	})

	t.Run("reconciles successfully when finalizer already exists", func(t *testing.T) {
		mockServiceRegistrationBackend := newMockServiceRegistrationBackend(t)
		mockStatusPatcher := newMockStatusPatcher(t)
		mockSecretReconciler := newMockSecretReconciler(t)
		recorder := &authRegistrationControllerClientRecorder{}
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		authRegistration.Finalizers = []string{authRegistrationFinalizer}
		key := types.NamespacedName{Name: authRegistration.Name, Namespace: authRegistration.Namespace}
		reconciler, c := newAuthRegistrationControllerReconcilerForTest(
			t,
			recorder,
			mockServiceRegistrationBackend,
			mockStatusPatcher,
			mockSecretReconciler,
			authRegistration,
		)
		registrationResult := newOIDCRegistrationResultForControllerTest()

		mockServiceRegistrationBackend.EXPECT().
			Upsert(mock.Anything, matchRegistration(domain.FromAuthRegistration(authRegistration))).
			Return(registrationResult, nil).
			Once()
		mockStatusPatcher.EXPECT().
			PatchResolvedSecretRef(mock.Anything, matchAuthRegistration("ecosystem", "auth-reg"), "auth-reg"+defaultGeneratedSecretSuffix).
			Return(nil).
			Once()
		mockSecretReconciler.EXPECT().
			Reconcile(
				mock.Anything,
				registrationResult,
				matchAuthRegistration("ecosystem", "auth-reg"),
				"auth-reg"+defaultGeneratedSecretSuffix,
				true,
			).
			Return(nil).
			Once()
		mockStatusPatcher.EXPECT().
			PatchCredentialsPublished(mock.Anything, matchAuthRegistration("ecosystem", "auth-reg")).
			Return(nil).
			Once()
		mockStatusPatcher.EXPECT().
			PatchRegistrationSucceeded(mock.Anything, matchAuthRegistration("ecosystem", "auth-reg")).
			Return(nil).
			Once()

		result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: key})

		require.NoError(t, err)
		assert.Equal(t, ctrl.Result{}, result)
		assert.Equal(t, 0, recorder.updateCalls)

		updated := getAuthRegistrationFromClientForControllerTest(t, c, key)
		assert.True(t, containsFinalizer(updated, authRegistrationFinalizer))
	})
}

func newAuthRegistrationControllerReconcilerForTest(
	t *testing.T,
	recorder *authRegistrationControllerClientRecorder,
	mockServiceRegistrationBackend *mockServiceRegistrationBackend,
	mockStatusPatcher *mockStatusPatcher,
	mockSecretReconciler *mockSecretReconciler,
	objects ...ctrlclient.Object,
) (*AuthRegistrationReconciler, ctrlclient.Client) {
	t.Helper()

	if recorder == nil {
		recorder = &authRegistrationControllerClientRecorder{}
	}

	builder := fake.NewClientBuilder().
		WithScheme(newAuthRegistrationControllerSchemeForTest(t)).
		WithObjects(objects...).
		WithInterceptorFuncs(interceptor.Funcs{
			Get:    recorder.interceptGet,
			Update: recorder.interceptUpdate,
			Delete: recorder.interceptDelete,
		})

	client := builder.Build()
	reconciler := &AuthRegistrationReconciler{
		Client:                      client,
		Scheme:                      newAuthRegistrationControllerSchemeForTest(t),
		credentialsSecretReconciler: mockSecretReconciler,
		statusPatcher:               mockStatusPatcher,
		serviceRegistrationBackend:  mockServiceRegistrationBackend,
	}

	return reconciler, client
}

func newAuthRegistrationControllerSchemeForTest(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, authregistrationv1.AddToScheme(scheme))

	return scheme
}

func newAuthRegistrationForControllerTest(namespace, name string) *authregistrationv1.AuthRegistration {
	return &authregistrationv1.AuthRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID(name + "-uid"),
		},
		Spec: authregistrationv1.AuthRegistrationSpec{
			Protocol: authregistrationv1.AuthProtocolOIDC,
			Consumer: "test-consumer",
			Params: map[string]string{
				"tenant": "dogu",
			},
		},
	}
}

func newOIDCRegistrationResultForControllerTest() domain.RegistrationResult {
	return domain.RegistrationResult{
		Protocol: domain.ProtocolOIDC,
		OIDC: &domain.OIDCResult{
			ClientID:     "oidc-client-id",
			ClientSecret: "oidc-client-secret",
			IssuerURL:    "https://issuer.example",
		},
	}
}

func getAuthRegistrationFromClientForControllerTest(
	t *testing.T,
	c ctrlclient.Client,
	key types.NamespacedName,
) *authregistrationv1.AuthRegistration {
	t.Helper()

	authRegistration := &authregistrationv1.AuthRegistration{}
	require.NoError(t, c.Get(context.Background(), key, authRegistration))

	return authRegistration
}

func stringPtrForControllerTest(value string) *string {
	return &value
}

func containsFinalizer(authRegistration *authregistrationv1.AuthRegistration, finalizer string) bool {
	for _, candidate := range authRegistration.Finalizers {
		if candidate == finalizer {
			return true
		}
	}

	return false
}

func matchRegistration(expected domain.Registration) interface{} {
	return mock.MatchedBy(func(actual domain.Registration) bool {
		return reflect.DeepEqual(expected, actual)
	})
}

func matchAuthRegistration(namespace, name string) interface{} {
	return mock.MatchedBy(func(actual *authregistrationv1.AuthRegistration) bool {
		return actual != nil && actual.Namespace == namespace && actual.Name == name
	})
}

type authRegistrationControllerClientRecorder struct {
	getCalls    int
	updateCalls int
	deleteCalls int

	getErr    error
	updateErr error
	deleteErr error
}

func (r *authRegistrationControllerClientRecorder) interceptGet(
	ctx context.Context,
	c ctrlclient.WithWatch,
	key ctrlclient.ObjectKey,
	obj ctrlclient.Object,
	opts ...ctrlclient.GetOption,
) error {
	r.getCalls++

	if r.getErr != nil {
		return r.getErr
	}

	return c.Get(ctx, key, obj, opts...)
}

func (r *authRegistrationControllerClientRecorder) interceptUpdate(
	ctx context.Context,
	c ctrlclient.WithWatch,
	obj ctrlclient.Object,
	opts ...ctrlclient.UpdateOption,
) error {
	r.updateCalls++

	if r.updateErr != nil {
		return r.updateErr
	}

	return c.Update(ctx, obj, opts...)
}

func (r *authRegistrationControllerClientRecorder) interceptDelete(
	ctx context.Context,
	c ctrlclient.WithWatch,
	obj ctrlclient.Object,
	opts ...ctrlclient.DeleteOption,
) error {
	r.deleteCalls++

	if r.deleteErr != nil {
		return r.deleteErr
	}

	return c.Delete(ctx, obj, opts...)
}

func newGeneratedSecretForControllerTest(authRegistration *authregistrationv1.AuthRegistration, secretName string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: authRegistration.Namespace,
			Annotations: map[string]string{
				generatedSecretAnnotationKey: "true",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: authregistrationv1.GroupVersion.String(),
					Kind:       "AuthRegistration",
					Name:       authRegistration.Name,
					UID:        authRegistration.UID,
					Controller: boolPtrForControllerTest(true),
				},
			},
		},
		Type: corev1.SecretTypeOpaque,
	}
}

func boolPtrForControllerTest(value bool) *bool {
	return &value
}
