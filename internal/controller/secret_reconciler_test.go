package controller

import (
	"context"
	"errors"
	"testing"

	authregistrationv1 "github.com/cloudogu/k8s-auth-registration-lib/api/v1"
	"github.com/cloudogu/k8s-auth-registration-operator/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestBuildDesiredSecret(t *testing.T) {
	t.Run("builds unmanaged secret with protocol and registration result data", func(t *testing.T) {
		authRegistration := newAuthRegistrationForSecretReconcilerTest(
			"ecosystem",
			"my-auth-registration",
			authregistrationv1.AuthProtocolOAuth,
		)
		registrationResult := newOIDCRegistrationResultForSecretReconcilerTest()

		secret := buildDesiredSecret(registrationResult, authRegistration, "target-secret", false)

		assert.Equal(t, "target-secret", secret.Name)
		assert.Equal(t, "ecosystem", secret.Namespace)
		assert.Equal(t, corev1.SecretTypeOpaque, secret.Type)
		assert.Equal(t, "my-auth-registration", secret.Labels[authRegistrationNameLabelKey])
		assert.Equal(t, []byte("oidc-client-id"), secret.Data["clientId"])
		assert.Equal(t, []byte("oidc-client-secret"), secret.Data["clientSecret"])
		assert.Equal(t, []byte("https://issuer.example"), secret.Data["issuerUrl"])
		assert.NotContains(t, secret.Annotations, generatedSecretAnnotationKey)
	})

	t.Run("adds generated-secret annotation for controller-managed secrets", func(t *testing.T) {
		authRegistration := newAuthRegistrationForSecretReconcilerTest(
			"ecosystem",
			"my-auth-registration",
			authregistrationv1.AuthProtocolOIDC,
		)
		registrationResult := newOIDCRegistrationResultForSecretReconcilerTest()

		secret := buildDesiredSecret(registrationResult, authRegistration, "target-secret", true)

		assert.Equal(t, "true", secret.Annotations[generatedSecretAnnotationKey])
	})
}

func TestAuthRegistrationSecretReconciler_Reconcile_CreatePaths(t *testing.T) {
	t.Run("creates unmanaged secret when the secret does not exist", func(t *testing.T) {
		scheme := newSecretReconcilerSchemeForTest(t, true)
		recorder := &secretReconcilerClientRecorder{}
		reconciler, c := newSecretReconcilerForTest(t, scheme, recorder)

		authRegistration := newAuthRegistrationForSecretReconcilerTest("ecosystem", "auth-reg", authregistrationv1.AuthProtocolOIDC)
		registrationResult := newOIDCRegistrationResultForSecretReconcilerTest()

		err := reconciler.Reconcile(context.Background(), registrationResult, authRegistration, "target-secret", false)

		require.NoError(t, err)
		assert.Equal(t, 1, recorder.createCalls)
		assert.Equal(t, 0, recorder.updateCalls)

		secret := getSecretFromClientForTest(t, c, types.NamespacedName{Name: "target-secret", Namespace: "ecosystem"})
		assert.Empty(t, secret.OwnerReferences)
		assert.NotContains(t, secret.Annotations, generatedSecretAnnotationKey)
		assert.Equal(t, []byte("oidc-client-id"), secret.Data["clientId"])
	})

	t.Run("creates managed secret with owner reference and generated annotation", func(t *testing.T) {
		scheme := newSecretReconcilerSchemeForTest(t, true)
		recorder := &secretReconcilerClientRecorder{}
		reconciler, c := newSecretReconcilerForTest(t, scheme, recorder)

		authRegistration := newAuthRegistrationForSecretReconcilerTest("ecosystem", "auth-reg", authregistrationv1.AuthProtocolOIDC)
		registrationResult := newOIDCRegistrationResultForSecretReconcilerTest()

		err := reconciler.Reconcile(context.Background(), registrationResult, authRegistration, "target-secret", true)

		require.NoError(t, err)
		assert.Equal(t, 1, recorder.createCalls)
		assert.Equal(t, 0, recorder.updateCalls)

		secret := getSecretFromClientForTest(t, c, types.NamespacedName{Name: "target-secret", Namespace: "ecosystem"})
		controllerRef := metav1.GetControllerOf(secret)
		require.NotNil(t, controllerRef)
		assert.Equal(t, "AuthRegistration", controllerRef.Kind)
		assert.Equal(t, "auth-reg", controllerRef.Name)
		assert.Equal(t, "k8s.cloudogu.com/v1", controllerRef.APIVersion)
		assert.Equal(t, "true", secret.Annotations[generatedSecretAnnotationKey])
	})

	t.Run("returns an error when setting owner reference on create fails", func(t *testing.T) {
		scheme := newSecretReconcilerSchemeForTest(t, false)
		recorder := &secretReconcilerClientRecorder{}
		reconciler, _ := newSecretReconcilerForTest(t, scheme, recorder)

		authRegistration := newAuthRegistrationForSecretReconcilerTest("ecosystem", "auth-reg", authregistrationv1.AuthProtocolOIDC)
		registrationResult := newOIDCRegistrationResultForSecretReconcilerTest()

		err := reconciler.Reconcile(context.Background(), registrationResult, authRegistration, "target-secret", true)

		require.Error(t, err)
		assert.ErrorContains(t, err, "no kind is registered for the type")
		assert.Equal(t, 0, recorder.createCalls)
		assert.Equal(t, 0, recorder.updateCalls)
	})

	t.Run("propagates create errors from the client", func(t *testing.T) {
		scheme := newSecretReconcilerSchemeForTest(t, true)
		recorder := &secretReconcilerClientRecorder{
			createErr: errors.New("create failed"),
		}
		reconciler, _ := newSecretReconcilerForTest(t, scheme, recorder)

		authRegistration := newAuthRegistrationForSecretReconcilerTest("ecosystem", "auth-reg", authregistrationv1.AuthProtocolOIDC)
		registrationResult := newOIDCRegistrationResultForSecretReconcilerTest()

		err := reconciler.Reconcile(context.Background(), registrationResult, authRegistration, "target-secret", false)

		require.Error(t, err)
		assert.EqualError(t, err, "create failed")
		assert.Equal(t, 1, recorder.createCalls)
		assert.Equal(t, 0, recorder.updateCalls)
	})
}

func TestAuthRegistrationSecretReconciler_Reconcile_UpdatePaths(t *testing.T) {
	t.Run("returns non-notfound get errors", func(t *testing.T) {
		scheme := newSecretReconcilerSchemeForTest(t, true)
		recorder := &secretReconcilerClientRecorder{
			getErr: errors.New("get failed"),
		}
		reconciler, _ := newSecretReconcilerForTest(t, scheme, recorder)

		authRegistration := newAuthRegistrationForSecretReconcilerTest("ecosystem", "auth-reg", authregistrationv1.AuthProtocolOIDC)
		registrationResult := newOIDCRegistrationResultForSecretReconcilerTest()

		err := reconciler.Reconcile(context.Background(), registrationResult, authRegistration, "target-secret", false)

		require.Error(t, err)
		assert.EqualError(t, err, "get failed")
		assert.Equal(t, 0, recorder.createCalls)
		assert.Equal(t, 0, recorder.updateCalls)
	})

	t.Run("updates existing secret and merges desired values while preserving unrelated keys", func(t *testing.T) {
		scheme := newSecretReconcilerSchemeForTest(t, true)
		recorder := &secretReconcilerClientRecorder{}
		existingSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "target-secret",
				Namespace: "ecosystem",
				Labels: map[string]string{
					"keep-label": "keep-value",
				},
				Annotations: map[string]string{
					"keep-annotation": "keep-value",
				},
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				"keep-data": []byte("keep-value"),
				"clientId":  []byte("old-client-id"),
			},
		}
		reconciler, c := newSecretReconcilerForTest(t, scheme, recorder, existingSecret)

		authRegistration := newAuthRegistrationForSecretReconcilerTest("ecosystem", "auth-reg", authregistrationv1.AuthProtocolOIDC)
		registrationResult := newOIDCRegistrationResultForSecretReconcilerTest()

		err := reconciler.Reconcile(context.Background(), registrationResult, authRegistration, "target-secret", false)

		require.NoError(t, err)
		assert.Equal(t, 0, recorder.createCalls)
		assert.Equal(t, 1, recorder.updateCalls)

		secret := getSecretFromClientForTest(t, c, types.NamespacedName{Name: "target-secret", Namespace: "ecosystem"})
		assert.Equal(t, corev1.SecretTypeOpaque, secret.Type)
		assert.Equal(t, "keep-value", secret.Labels["keep-label"])
		assert.Equal(t, "auth-reg", secret.Labels[authRegistrationNameLabelKey])
		assert.Equal(t, "keep-value", secret.Annotations["keep-annotation"])
		assert.Equal(t, []byte("keep-value"), secret.Data["keep-data"])
		assert.Equal(t, []byte("oidc-client-id"), secret.Data["clientId"])
		assert.Equal(t, []byte("oidc-client-secret"), secret.Data["clientSecret"])
		assert.Equal(t, []byte("https://issuer.example"), secret.Data["issuerUrl"])
	})

	t.Run("initializes nil label, annotation and data maps during update", func(t *testing.T) {
		scheme := newSecretReconcilerSchemeForTest(t, true)
		recorder := &secretReconcilerClientRecorder{}
		existingSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "target-secret",
				Namespace: "ecosystem",
			},
			Type: corev1.SecretTypeTLS,
		}
		reconciler, c := newSecretReconcilerForTest(t, scheme, recorder, existingSecret)

		authRegistration := newAuthRegistrationForSecretReconcilerTest("ecosystem", "auth-reg", authregistrationv1.AuthProtocolOIDC)
		registrationResult := newOIDCRegistrationResultForSecretReconcilerTest()

		err := reconciler.Reconcile(context.Background(), registrationResult, authRegistration, "target-secret", false)

		require.NoError(t, err)
		assert.Equal(t, 1, recorder.updateCalls)

		secret := getSecretFromClientForTest(t, c, types.NamespacedName{Name: "target-secret", Namespace: "ecosystem"})
		require.NotNil(t, secret.Labels)
		assert.Empty(t, secret.Annotations)
		require.NotNil(t, secret.Data)
		assert.Equal(t, "auth-reg", secret.Labels[authRegistrationNameLabelKey])
		assert.Equal(t, []byte("oidc-client-id"), secret.Data["clientId"])
	})

	t.Run("does not issue update when existing secret already matches desired state", func(t *testing.T) {
		scheme := newSecretReconcilerSchemeForTest(t, true)
		recorder := &secretReconcilerClientRecorder{}
		authRegistration := newAuthRegistrationForSecretReconcilerTest("ecosystem", "auth-reg", authregistrationv1.AuthProtocolOIDC)
		registrationResult := newOIDCRegistrationResultForSecretReconcilerTest()
		existingSecret := buildDesiredSecret(registrationResult, authRegistration, "target-secret", false)
		existingSecret.Annotations = map[string]string{}

		reconciler, _ := newSecretReconcilerForTest(t, scheme, recorder, existingSecret)

		err := reconciler.Reconcile(context.Background(), registrationResult, authRegistration, "target-secret", false)

		require.NoError(t, err)
		assert.Equal(t, 0, recorder.createCalls)
		assert.Equal(t, 0, recorder.updateCalls)
	})

	t.Run("sets owner reference during update when secret is controller managed", func(t *testing.T) {
		scheme := newSecretReconcilerSchemeForTest(t, true)
		recorder := &secretReconcilerClientRecorder{}
		existingSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "target-secret",
				Namespace:   "ecosystem",
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{},
		}
		reconciler, c := newSecretReconcilerForTest(t, scheme, recorder, existingSecret)

		authRegistration := newAuthRegistrationForSecretReconcilerTest("ecosystem", "auth-reg", authregistrationv1.AuthProtocolOIDC)
		registrationResult := newOIDCRegistrationResultForSecretReconcilerTest()

		err := reconciler.Reconcile(context.Background(), registrationResult, authRegistration, "target-secret", true)

		require.NoError(t, err)
		assert.Equal(t, 1, recorder.updateCalls)

		secret := getSecretFromClientForTest(t, c, types.NamespacedName{Name: "target-secret", Namespace: "ecosystem"})
		controllerRef := metav1.GetControllerOf(secret)
		require.NotNil(t, controllerRef)
		assert.Equal(t, "AuthRegistration", controllerRef.Kind)
		assert.Equal(t, "auth-reg", controllerRef.Name)
		assert.Equal(t, "true", secret.Annotations[generatedSecretAnnotationKey])
	})

	t.Run("returns an error when setting owner reference on update fails", func(t *testing.T) {
		scheme := newSecretReconcilerSchemeForTest(t, false)
		recorder := &secretReconcilerClientRecorder{}
		existingSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "target-secret",
				Namespace: "ecosystem",
			},
		}
		reconciler, _ := newSecretReconcilerForTest(t, scheme, recorder, existingSecret)

		authRegistration := newAuthRegistrationForSecretReconcilerTest("ecosystem", "auth-reg", authregistrationv1.AuthProtocolOIDC)
		registrationResult := newOIDCRegistrationResultForSecretReconcilerTest()

		err := reconciler.Reconcile(context.Background(), registrationResult, authRegistration, "target-secret", true)

		require.Error(t, err)
		assert.ErrorContains(t, err, "no kind is registered for the type")
		assert.Equal(t, 0, recorder.updateCalls)
	})

	t.Run("propagates update errors when the secret changed", func(t *testing.T) {
		scheme := newSecretReconcilerSchemeForTest(t, true)
		recorder := &secretReconcilerClientRecorder{
			updateErr: errors.New("update failed"),
		}
		existingSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "target-secret",
				Namespace: "ecosystem",
			},
		}
		reconciler, _ := newSecretReconcilerForTest(t, scheme, recorder, existingSecret)

		authRegistration := newAuthRegistrationForSecretReconcilerTest("ecosystem", "auth-reg", authregistrationv1.AuthProtocolOIDC)
		registrationResult := newOIDCRegistrationResultForSecretReconcilerTest()

		err := reconciler.Reconcile(context.Background(), registrationResult, authRegistration, "target-secret", false)

		require.Error(t, err)
		assert.EqualError(t, err, "update failed")
		assert.Equal(t, 1, recorder.updateCalls)
	})
}

func newSecretReconcilerForTest(
	t *testing.T,
	scheme *runtime.Scheme,
	recorder *secretReconcilerClientRecorder,
	objects ...ctrlclient.Object,
) (*authRegistrationSecretReconciler, ctrlclient.Client) {
	t.Helper()

	if recorder == nil {
		recorder = &secretReconcilerClientRecorder{}
	}

	builder := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		WithInterceptorFuncs(interceptor.Funcs{
			Get:    recorder.interceptGet,
			Create: recorder.interceptCreate,
			Update: recorder.interceptUpdate,
		})

	c := builder.Build()
	reconciler := &authRegistrationSecretReconciler{
		Client: c,
		Scheme: scheme,
	}

	return reconciler, c
}

func newSecretReconcilerSchemeForTest(t *testing.T, includeAuthRegistration bool) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	if includeAuthRegistration {
		require.NoError(t, authregistrationv1.AddToScheme(scheme))
	}

	return scheme
}

func newAuthRegistrationForSecretReconcilerTest(namespace, name string, protocol authregistrationv1.AuthProtocol) *authregistrationv1.AuthRegistration {
	return &authregistrationv1.AuthRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID(name + "-uid"),
		},
		Spec: authregistrationv1.AuthRegistrationSpec{
			Protocol: protocol,
			Consumer: "test-consumer",
		},
	}
}

func newOIDCRegistrationResultForSecretReconcilerTest() domain.RegistrationResult {
	return domain.RegistrationResult{
		Protocol: domain.ProtocolOIDC,
		OIDC: &domain.OIDCResult{
			ClientID:     "oidc-client-id",
			ClientSecret: "oidc-client-secret",
			IssuerURL:    "https://issuer.example",
		},
	}
}

func getSecretFromClientForTest(t *testing.T, c ctrlclient.Client, key types.NamespacedName) *corev1.Secret {
	t.Helper()

	secret := &corev1.Secret{}
	require.NoError(t, c.Get(context.Background(), key, secret))

	return secret
}

type secretReconcilerClientRecorder struct {
	getCalls    int
	createCalls int
	updateCalls int

	getErr    error
	createErr error
	updateErr error
}

func (r *secretReconcilerClientRecorder) interceptGet(
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

func (r *secretReconcilerClientRecorder) interceptCreate(
	ctx context.Context,
	c ctrlclient.WithWatch,
	obj ctrlclient.Object,
	opts ...ctrlclient.CreateOption,
) error {
	r.createCalls++

	if r.createErr != nil {
		return r.createErr
	}

	return c.Create(ctx, obj, opts...)
}

func (r *secretReconcilerClientRecorder) interceptUpdate(
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
