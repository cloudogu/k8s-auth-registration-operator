package controller

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	authregistrationv1 "github.com/cloudogu/k8s-auth-registration-lib/api/v1"
	"github.com/cloudogu/k8s-auth-registration-operator/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
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

func TestIndexAuthRegistrationBySecretName(t *testing.T) {
	t.Run("indexes generated secret name when spec.secretRef is nil", func(t *testing.T) {
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")

		indexValues := indexAuthRegistrationBySecretName(authRegistration)

		assert.Equal(t, []string{"auth-reg" + defaultGeneratedSecretSuffix}, indexValues)
	})

	t.Run("indexes trimmed explicit secretRef", func(t *testing.T) {
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		authRegistration.Spec.SecretRef = stringPtrForControllerTest("  target-secret  ")

		indexValues := indexAuthRegistrationBySecretName(authRegistration)

		assert.Equal(t, []string{"target-secret"}, indexValues)
	})

	t.Run("returns nil for invalid empty secretRef", func(t *testing.T) {
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		authRegistration.Spec.SecretRef = stringPtrForControllerTest("  \n\t ")

		indexValues := indexAuthRegistrationBySecretName(authRegistration)

		assert.Nil(t, indexValues)
	})

	t.Run("returns nil for non authregistration objects", func(t *testing.T) {
		indexValues := indexAuthRegistrationBySecretName(&corev1.Secret{})

		assert.Nil(t, indexValues)
	})
}

func TestAuthRegistrationReconciler_MapSecretToAuthRegistrations(t *testing.T) {
	scheme := newAuthRegistrationControllerSchemeForTest(t)
	defaultRefAuthRegistration := newAuthRegistrationForControllerTest("ecosystem", "default-auth-reg")
	explicitRefAuthRegistration := newAuthRegistrationForControllerTest("ecosystem", "explicit-auth-reg")
	explicitRefAuthRegistration.Spec.SecretRef = stringPtrForControllerTest("shared-secret")
	otherRefAuthRegistration := newAuthRegistrationForControllerTest("ecosystem", "other-auth-reg")
	otherRefAuthRegistration.Spec.SecretRef = stringPtrForControllerTest("other-secret")
	otherNamespaceAuthRegistration := newAuthRegistrationForControllerTest("other", "other-namespace-auth-reg")
	otherNamespaceAuthRegistration.Spec.SecretRef = stringPtrForControllerTest("shared-secret")

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			defaultRefAuthRegistration,
			explicitRefAuthRegistration,
			otherRefAuthRegistration,
			otherNamespaceAuthRegistration,
		).
		WithIndex(&authregistrationv1.AuthRegistration{}, authRegistrationSecretRefField, indexAuthRegistrationBySecretName).
		Build()

	reconciler := &AuthRegistrationController{
		Client: client,
	}

	t.Run("maps explicit secretRef to matching auth registration in same namespace", func(t *testing.T) {
		requests := reconciler.mapSecretToAuthRegistrations(context.Background(), &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ecosystem",
				Name:      "shared-secret",
			},
		})

		assert.ElementsMatch(t, []ctrl.Request{
			{
				NamespacedName: types.NamespacedName{
					Namespace: "ecosystem",
					Name:      "explicit-auth-reg",
				},
			},
		}, requests)
	})

	t.Run("maps generated secret name to matching auth registration", func(t *testing.T) {
		requests := reconciler.mapSecretToAuthRegistrations(context.Background(), &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ecosystem",
				Name:      "default-auth-reg" + defaultGeneratedSecretSuffix,
			},
		})

		assert.ElementsMatch(t, []ctrl.Request{
			{
				NamespacedName: types.NamespacedName{
					Namespace: "ecosystem",
					Name:      "default-auth-reg",
				},
			},
		}, requests)
	})

	t.Run("returns no requests for unrelated secret", func(t *testing.T) {
		requests := reconciler.mapSecretToAuthRegistrations(context.Background(), &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ecosystem",
				Name:      "unknown-secret",
			},
		})

		assert.Empty(t, requests)
	})

	t.Run("returns no requests when watched object is not a secret", func(t *testing.T) {
		requests := reconciler.mapSecretToAuthRegistrations(context.Background(), defaultRefAuthRegistration)

		assert.Empty(t, requests)
	})

	t.Run("returns no requests when listing authregistrations fails", func(t *testing.T) {
		listErr := errors.New("list failed")
		clientWithListError := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(
				defaultRefAuthRegistration,
				explicitRefAuthRegistration,
				otherRefAuthRegistration,
				otherNamespaceAuthRegistration,
			).
			WithIndex(&authregistrationv1.AuthRegistration{}, authRegistrationSecretRefField, indexAuthRegistrationBySecretName).
			WithInterceptorFuncs(interceptor.Funcs{
				List: func(ctx context.Context, c ctrlclient.WithWatch, list ctrlclient.ObjectList, opts ...ctrlclient.ListOption) error {
					return listErr
				},
			}).
			Build()

		controllerWithListError := &AuthRegistrationController{
			Client: clientWithListError,
		}

		requests := controllerWithListError.mapSecretToAuthRegistrations(context.Background(), &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ecosystem",
				Name:      "shared-secret",
			},
		})

		assert.Empty(t, requests)
	})
}

func TestNewAuthRegistrationController(t *testing.T) {
	t.Run("constructs reconciler with default collaborators and provided backend", func(t *testing.T) {
		scheme := newAuthRegistrationControllerSchemeForTest(t)
		client := fake.NewClientBuilder().WithScheme(scheme).Build()
		mockServiceRegistrationBackend := newMockServiceRegistrationBackend(t)

		reconciler := NewAuthRegistrationController(client, scheme, mockServiceRegistrationBackend)
		defaultReconciler, hasDefaultReconciler := reconciler.reconciler.(*defaultAuthRegistrationReconciler)

		require.NotNil(t, reconciler)
		assert.Same(t, client, reconciler.Client)
		assert.True(t, hasDefaultReconciler)
		assert.Same(t, mockServiceRegistrationBackend, defaultReconciler.serviceRegistrationBackend)
		_, hasDefaultSecretReconciler := defaultReconciler.credentialsSecretReconciler.(*authRegistrationSecretReconciler)
		_, hasDefaultStatusPatcher := defaultReconciler.statusPatcher.(*authRegistrationStatusPatcher)
		assert.True(t, hasDefaultSecretReconciler)
		assert.True(t, hasDefaultStatusPatcher)
	})
}

func TestAuthRegistrationController_Reconcile(t *testing.T) {
	t.Run("returns nil when resource is not found", func(t *testing.T) {
		mockAuthReconciler := newMockAuthRegistrationReconciler(t)
		controller, _ := newAuthRegistrationControllerForTest(t, nil, mockAuthReconciler)
		request := ctrl.Request{NamespacedName: types.NamespacedName{Name: "auth-reg", Namespace: "ecosystem"}}

		result, err := controller.Reconcile(context.Background(), request)

		require.NoError(t, err)
		assert.Equal(t, ctrl.Result{}, result)
	})

	t.Run("returns non-notfound get errors", func(t *testing.T) {
		mockAuthReconciler := newMockAuthRegistrationReconciler(t)
		recorder := &authRegistrationControllerClientRecorder{
			getErr: errors.New("get failed"),
		}
		controller, _ := newAuthRegistrationControllerForTest(t, recorder, mockAuthReconciler)
		request := ctrl.Request{NamespacedName: types.NamespacedName{Name: "auth-reg", Namespace: "ecosystem"}}

		result, err := controller.Reconcile(context.Background(), request)

		require.Error(t, err)
		assert.EqualError(t, err, "get failed")
		assert.Equal(t, ctrl.Result{}, result)
	})

	t.Run("returns update error when adding finalizer fails", func(t *testing.T) {
		mockAuthReconciler := newMockAuthRegistrationReconciler(t)
		recorder := &authRegistrationControllerClientRecorder{
			updateErr: errors.New("update failed"),
		}
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		controller, _ := newAuthRegistrationControllerForTest(t, recorder, mockAuthReconciler, authRegistration)
		request := ctrl.Request{NamespacedName: types.NamespacedName{Name: "auth-reg", Namespace: "ecosystem"}}

		result, err := controller.Reconcile(context.Background(), request)

		require.Error(t, err)
		assert.EqualError(t, err, "update failed")
		assert.Equal(t, ctrl.Result{}, result)
	})

	t.Run("uses deletion flow when deletion timestamp is set", func(t *testing.T) {
		mockAuthReconciler := newMockAuthRegistrationReconciler(t)
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		authRegistration.Finalizers = []string{authRegistrationFinalizer}
		deletionTime := metav1.NewTime(time.Now())
		authRegistration.DeletionTimestamp = &deletionTime
		key := types.NamespacedName{Name: authRegistration.Name, Namespace: authRegistration.Namespace}
		controller, _ := newAuthRegistrationControllerForTest(t, nil, mockAuthReconciler, authRegistration)
		expectedResult := ctrl.Result{Requeue: true}
		expectedErr := errors.New("delete delegated error")

		mockAuthReconciler.EXPECT().
			handleDeletion(mock.Anything, matchAuthRegistration("ecosystem", "auth-reg"), mock.Anything).
			Return(expectedResult, expectedErr).
			Once()

		result, err := controller.Reconcile(context.Background(), ctrl.Request{NamespacedName: key})

		require.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.Equal(t, expectedResult, result)
	})

	t.Run("persists finalizer before returning handleReconcile errors", func(t *testing.T) {
		mockAuthReconciler := newMockAuthRegistrationReconciler(t)
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		key := types.NamespacedName{Name: authRegistration.Name, Namespace: authRegistration.Namespace}
		controller, c := newAuthRegistrationControllerForTest(t, nil, mockAuthReconciler, authRegistration)
		reconcileErr := errors.New("reconcile failed")

		mockAuthReconciler.EXPECT().
			handleReconcile(mock.Anything, matchAuthRegistration("ecosystem", "auth-reg"), mock.Anything).
			Return(reconcileErr).
			Once()

		result, err := controller.Reconcile(context.Background(), ctrl.Request{NamespacedName: key})

		require.Error(t, err)
		assert.Equal(t, ctrl.Result{}, result)
		assert.ErrorContains(t, err, "reconcile failed")

		updated := getAuthRegistrationFromClientForControllerTest(t, c, key)
		assert.True(t, containsFinalizer(updated, authRegistrationFinalizer))
	})

	t.Run("reconciles successfully when finalizer already exists", func(t *testing.T) {
		mockAuthReconciler := newMockAuthRegistrationReconciler(t)
		recorder := &authRegistrationControllerClientRecorder{}
		authRegistration := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		authRegistration.Finalizers = []string{authRegistrationFinalizer}
		key := types.NamespacedName{Name: authRegistration.Name, Namespace: authRegistration.Namespace}
		controller, c := newAuthRegistrationControllerForTest(t, recorder, mockAuthReconciler, authRegistration)

		mockAuthReconciler.EXPECT().
			handleReconcile(mock.Anything, matchAuthRegistration("ecosystem", "auth-reg"), mock.Anything).
			Return(nil).
			Once()

		result, err := controller.Reconcile(context.Background(), ctrl.Request{NamespacedName: key})

		require.NoError(t, err)
		assert.Equal(t, ctrl.Result{}, result)
		assert.Equal(t, 0, recorder.updateCalls)

		updated := getAuthRegistrationFromClientForControllerTest(t, c, key)
		assert.True(t, containsFinalizer(updated, authRegistrationFinalizer))
	})
}

func newAuthRegistrationControllerForTest(
	t *testing.T,
	recorder *authRegistrationControllerClientRecorder,
	mockAuthRegistrationReconciler *mockAuthRegistrationReconciler,
	objects ...ctrlclient.Object,
) (*AuthRegistrationController, ctrlclient.Client) {
	t.Helper()

	if recorder == nil {
		recorder = &authRegistrationControllerClientRecorder{}
	}
	if mockAuthRegistrationReconciler == nil {
		mockAuthRegistrationReconciler = newMockAuthRegistrationReconciler(t)
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
	reconciler := &AuthRegistrationController{
		Client:     client,
		reconciler: mockAuthRegistrationReconciler,
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
			Name:        secretName,
			Namespace:   authRegistration.Namespace,
			Annotations: map[string]string{},
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
