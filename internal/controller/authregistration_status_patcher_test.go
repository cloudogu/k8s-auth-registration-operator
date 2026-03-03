package controller

import (
	"context"
	"errors"
	"testing"

	authregistrationv1 "github.com/cloudogu/k8s-auth-registration-lib/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestAuthRegistrationStatusPatcher_PatchResolvedSecretRef(t *testing.T) {
	t.Run("sets status.resolvedSecretRef and persists it with one status patch", func(t *testing.T) {
		patcher, c, authRegistration, key, recorder := newStatusPatcherTestFixture(
			t,
			authregistrationv1.AuthRegistrationStatus{},
			3,
			nil,
		)

		err := patcher.PatchResolvedSecretRef(context.Background(), authRegistration, "resolved-secret")

		require.NoError(t, err)
		assert.Equal(t, 1, recorder.statusPatchCalls)

		got := getAuthRegistrationFromClient(t, c, key)
		assert.Equal(t, "resolved-secret", got.Status.ResolvedSecretRef)
	})
}

func TestAuthRegistrationStatusPatcher_PatchInvalidSpec(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		initialStatus := authregistrationv1.AuthRegistrationStatus{
			ResolvedSecretRef: "old-secret",
		}
		patcher, c, authRegistration, key, recorder := newStatusPatcherTestFixture(
			t,
			initialStatus,
			11,
			nil,
		)

		err := patcher.PatchInvalidSpec(context.Background(), authRegistration, errors.New("spec is invalid"))

		require.NoError(t, err)
		assert.Equal(t, 3, recorder.statusPatchCalls)

		got := getAuthRegistrationFromClient(t, c, key)
		assert.Equal(t, "", got.Status.ResolvedSecretRef)
		assertCondition(
			t,
			got,
			authregistrationv1.ConditionCredentialsPublished,
			metav1.ConditionFalse,
			"InvalidSpec",
			"spec is invalid",
			11,
		)
		assertCondition(
			t,
			got,
			authregistrationv1.ConditionCompleted,
			metav1.ConditionFalse,
			"InvalidSpec",
			"Registration is blocked because the resource specification is invalid",
			11,
		)
		assert.Len(t, got.Status.Conditions, 2)
	})

	t.Run("fails when clearing resolved secret ref fails", func(t *testing.T) {
		initialStatus := authregistrationv1.AuthRegistrationStatus{
			ResolvedSecretRef: "old-secret",
		}
		patcher, c, authRegistration, key, recorder := newStatusPatcherTestFixture(
			t,
			initialStatus,
			15,
			errors.New("patch failed"), 1,
		)

		err := patcher.PatchInvalidSpec(context.Background(), authRegistration, errors.New("invalid spec"))

		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to clear resolved secret reference")
		assert.ErrorContains(t, err, "patch failed")
		assert.Equal(t, 1, recorder.statusPatchCalls)

		got := getAuthRegistrationFromClient(t, c, key)
		assert.Equal(t, "old-secret", got.Status.ResolvedSecretRef)
		assert.Empty(t, got.Status.Conditions)
	})

	t.Run("fails when credentials condition patch fails", func(t *testing.T) {
		initialStatus := authregistrationv1.AuthRegistrationStatus{
			ResolvedSecretRef: "old-secret",
		}
		patcher, c, authRegistration, key, recorder := newStatusPatcherTestFixture(
			t,
			initialStatus,
			16,
			errors.New("patch failed"), 2,
		)

		err := patcher.PatchInvalidSpec(context.Background(), authRegistration, errors.New("invalid spec"))

		require.Error(t, err)
		assert.EqualError(t, err, "patch failed")
		assert.Equal(t, 2, recorder.statusPatchCalls)

		got := getAuthRegistrationFromClient(t, c, key)
		assert.Equal(t, "", got.Status.ResolvedSecretRef)
		assert.Empty(t, got.Status.Conditions)
	})

	t.Run("fails when completed condition patch fails", func(t *testing.T) {
		initialStatus := authregistrationv1.AuthRegistrationStatus{
			ResolvedSecretRef: "old-secret",
		}
		patcher, c, authRegistration, key, recorder := newStatusPatcherTestFixture(
			t,
			initialStatus,
			17,
			errors.New("patch failed"), 3,
		)

		err := patcher.PatchInvalidSpec(context.Background(), authRegistration, errors.New("invalid spec"))

		require.Error(t, err)
		assert.EqualError(t, err, "patch failed")
		assert.Equal(t, 3, recorder.statusPatchCalls)

		got := getAuthRegistrationFromClient(t, c, key)
		assert.Equal(t, "", got.Status.ResolvedSecretRef)
		assertCondition(
			t,
			got,
			authregistrationv1.ConditionCredentialsPublished,
			metav1.ConditionFalse,
			"InvalidSpec",
			"invalid spec",
			17,
		)
		assert.Nil(t, meta.FindStatusCondition(got.Status.Conditions, authregistrationv1.ConditionCompleted))
		assert.Len(t, got.Status.Conditions, 1)
	})
}

func TestAuthRegistrationStatusPatcher_PatchSecretReconcileFailed(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		initialStatus := authregistrationv1.AuthRegistrationStatus{
			ResolvedSecretRef: "existing-secret",
		}
		patcher, c, authRegistration, key, recorder := newStatusPatcherTestFixture(
			t,
			initialStatus,
			7,
			nil,
		)

		err := patcher.PatchSecretReconcileFailed(
			context.Background(),
			authRegistration,
			"target-secret",
			errors.New("secret boom"),
		)

		require.NoError(t, err)
		assert.Equal(t, 2, recorder.statusPatchCalls)

		got := getAuthRegistrationFromClient(t, c, key)
		assert.Equal(t, "existing-secret", got.Status.ResolvedSecretRef)
		assertCondition(
			t,
			got,
			authregistrationv1.ConditionCredentialsPublished,
			metav1.ConditionFalse,
			"SecretReconcileFailed",
			"Failed to reconcile Secret \"target-secret\": secret boom",
			7,
		)
		assertCondition(
			t,
			got,
			authregistrationv1.ConditionCompleted,
			metav1.ConditionFalse,
			"Blocked",
			"Registration is blocked until Secret reconciliation succeeds",
			7,
		)
		assert.Len(t, got.Status.Conditions, 2)
	})

	t.Run("fails when credentials condition patch fails", func(t *testing.T) {
		initialStatus := authregistrationv1.AuthRegistrationStatus{
			ResolvedSecretRef: "existing-secret",
		}
		patcher, c, authRegistration, key, recorder := newStatusPatcherTestFixture(
			t,
			initialStatus,
			21,
			errors.New("patch failed"), 1,
		)

		err := patcher.PatchSecretReconcileFailed(
			context.Background(),
			authRegistration,
			"target-secret",
			errors.New("secret boom"),
		)

		require.Error(t, err)
		assert.EqualError(t, err, "patch failed")
		assert.Equal(t, 1, recorder.statusPatchCalls)

		got := getAuthRegistrationFromClient(t, c, key)
		assert.Equal(t, "existing-secret", got.Status.ResolvedSecretRef)
		assert.Empty(t, got.Status.Conditions)
	})

	t.Run("fails when completed condition patch fails", func(t *testing.T) {
		initialStatus := authregistrationv1.AuthRegistrationStatus{
			ResolvedSecretRef: "existing-secret",
		}
		patcher, c, authRegistration, key, recorder := newStatusPatcherTestFixture(
			t,
			initialStatus,
			22,
			errors.New("patch failed"), 2,
		)

		err := patcher.PatchSecretReconcileFailed(
			context.Background(),
			authRegistration,
			"target-secret",
			errors.New("secret boom"),
		)

		require.Error(t, err)
		assert.EqualError(t, err, "patch failed")
		assert.Equal(t, 2, recorder.statusPatchCalls)

		got := getAuthRegistrationFromClient(t, c, key)
		assert.Equal(t, "existing-secret", got.Status.ResolvedSecretRef)
		assertCondition(
			t,
			got,
			authregistrationv1.ConditionCredentialsPublished,
			metav1.ConditionFalse,
			"SecretReconcileFailed",
			"Failed to reconcile Secret \"target-secret\": secret boom",
			22,
		)
		assert.Nil(t, meta.FindStatusCondition(got.Status.Conditions, authregistrationv1.ConditionCompleted))
		assert.Len(t, got.Status.Conditions, 1)
	})
}

func TestAuthRegistrationStatusPatcher_PatchCredentialsPublished(t *testing.T) {
	t.Run("writes credentials published condition as true", func(t *testing.T) {
		patcher, c, authRegistration, key, recorder := newStatusPatcherTestFixture(
			t,
			authregistrationv1.AuthRegistrationStatus{},
			4,
			nil,
		)

		err := patcher.PatchCredentialsPublished(context.Background(), authRegistration)

		require.NoError(t, err)
		assert.Equal(t, 1, recorder.statusPatchCalls)

		got := getAuthRegistrationFromClient(t, c, key)
		assertCondition(
			t,
			got,
			authregistrationv1.ConditionCredentialsPublished,
			metav1.ConditionTrue,
			"SecretReconciled",
			"Credentials Secret is ready",
			4,
		)
		assert.Len(t, got.Status.Conditions, 1)
	})
}

func TestAuthRegistrationStatusPatcher_PatchRegistrationFailed(t *testing.T) {
	t.Run("writes completed=false condition with registration failure details", func(t *testing.T) {
		patcher, c, authRegistration, key, recorder := newStatusPatcherTestFixture(
			t,
			authregistrationv1.AuthRegistrationStatus{},
			8,
			nil,
		)

		err := patcher.PatchRegistrationFailed(context.Background(), authRegistration, errors.New("backend unavailable"))

		require.NoError(t, err)
		assert.Equal(t, 1, recorder.statusPatchCalls)

		got := getAuthRegistrationFromClient(t, c, key)
		assertCondition(
			t,
			got,
			authregistrationv1.ConditionCompleted,
			metav1.ConditionFalse,
			"RegistrationFailed",
			"Failed to register service in backend: backend unavailable",
			8,
		)
		assert.Len(t, got.Status.Conditions, 1)
	})
}

func TestAuthRegistrationStatusPatcher_PatchRegistrationSucceeded(t *testing.T) {
	t.Run("writes completed=true condition after successful registration", func(t *testing.T) {
		patcher, c, authRegistration, key, recorder := newStatusPatcherTestFixture(
			t,
			authregistrationv1.AuthRegistrationStatus{},
			9,
			nil,
		)

		err := patcher.PatchRegistrationSucceeded(context.Background(), authRegistration)

		require.NoError(t, err)
		assert.Equal(t, 1, recorder.statusPatchCalls)

		got := getAuthRegistrationFromClient(t, c, key)
		assertCondition(
			t,
			got,
			authregistrationv1.ConditionCompleted,
			metav1.ConditionTrue,
			"RegistrationSucceeded",
			"Service registration was reconciled successfully",
			9,
		)
		assert.Len(t, got.Status.Conditions, 1)
	})
}

func TestAuthRegistrationStatusPatcher_PatchCondition_EmptyType(t *testing.T) {
	t.Run("returns an error when condition type is empty and does not patch", func(t *testing.T) {
		initialStatus := authregistrationv1.AuthRegistrationStatus{
			ResolvedSecretRef: "unchanged",
		}
		patcher, c, authRegistration, key, recorder := newStatusPatcherTestFixture(
			t,
			initialStatus,
			5,
			nil,
		)

		err := patcher.patchCondition(context.Background(), authRegistration, metav1.Condition{})

		require.Error(t, err)
		assert.EqualError(t, err, "condition type must not be empty")
		assert.Equal(t, 0, recorder.statusPatchCalls)

		got := getAuthRegistrationFromClient(t, c, key)
		assert.Equal(t, "unchanged", got.Status.ResolvedSecretRef)
		assert.Empty(t, got.Status.Conditions)
	})
}

func TestAuthRegistrationStatusPatcher_PatchStatus_NoChange(t *testing.T) {
	t.Run("skips status patch when the resulting status is unchanged", func(t *testing.T) {
		initialStatus := authregistrationv1.AuthRegistrationStatus{
			ResolvedSecretRef: "already-set",
		}
		patcher, c, authRegistration, key, recorder := newStatusPatcherTestFixture(
			t,
			initialStatus,
			2,
			nil,
		)

		err := patcher.PatchResolvedSecretRef(context.Background(), authRegistration, "already-set")

		require.NoError(t, err)
		assert.Equal(t, 0, recorder.statusPatchCalls)

		got := getAuthRegistrationFromClient(t, c, key)
		assert.Equal(t, "already-set", got.Status.ResolvedSecretRef)
		assert.Empty(t, got.Status.Conditions)
	})
}

func TestAuthRegistrationStatusPatcher_PropagatesPatchError(t *testing.T) {
	t.Run("returns patch errors unchanged and keeps status in storage untouched", func(t *testing.T) {
		initialStatus := authregistrationv1.AuthRegistrationStatus{
			ResolvedSecretRef: "stable-secret",
		}
		patcher, c, authRegistration, key, recorder := newStatusPatcherTestFixture(
			t,
			initialStatus,
			1,
			errors.New("patch failed"),
		)

		err := patcher.PatchCredentialsPublished(context.Background(), authRegistration)

		require.Error(t, err)
		assert.EqualError(t, err, "patch failed")
		assert.Equal(t, 1, recorder.statusPatchCalls)

		got := getAuthRegistrationFromClient(t, c, key)
		assert.Equal(t, "stable-secret", got.Status.ResolvedSecretRef)
		assert.Empty(t, got.Status.Conditions)
	})
}

func assertCondition(
	t *testing.T,
	authRegistration *authregistrationv1.AuthRegistration,
	conditionType string,
	expectedStatus metav1.ConditionStatus,
	expectedReason string,
	expectedMessage string,
	expectedObservedGeneration int64,
) {
	t.Helper()

	condition := meta.FindStatusCondition(authRegistration.Status.Conditions, conditionType)
	require.NotNilf(t, condition, "expected condition %q to exist", conditionType)

	assert.Equal(t, conditionType, condition.Type)
	assert.Equal(t, expectedStatus, condition.Status)
	assert.Equal(t, expectedReason, condition.Reason)
	assert.Equal(t, expectedMessage, condition.Message)
	assert.Equal(t, expectedObservedGeneration, condition.ObservedGeneration)
	assert.False(t, condition.LastTransitionTime.IsZero())
}

func newStatusPatcherTestFixture(
	t *testing.T,
	initialStatus authregistrationv1.AuthRegistrationStatus,
	generation int64,
	statusPatchErr error,
	failOnStatusPatchCall ...int,
) (
	*authRegistrationStatusPatcher,
	ctrlclient.Client,
	*authregistrationv1.AuthRegistration,
	types.NamespacedName,
	*statusPatchRecorder,
) {
	t.Helper()

	failOnCall := 0
	if len(failOnStatusPatchCall) > 0 {
		failOnCall = failOnStatusPatchCall[0]
	}
	if statusPatchErr != nil && failOnCall == 0 {
		failOnCall = 1
	}

	scheme := runtime.NewScheme()
	require.NoError(t, authregistrationv1.AddToScheme(scheme))

	authRegistration := &authregistrationv1.AuthRegistration{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-auth-registration",
			Namespace:  "default",
			Generation: generation,
		},
		Spec: authregistrationv1.AuthRegistrationSpec{
			Protocol: authregistrationv1.AuthProtocolOIDC,
			Consumer: "test-consumer",
		},
		Status: initialStatus,
	}
	key := types.NamespacedName{
		Name:      authRegistration.Name,
		Namespace: authRegistration.Namespace,
	}

	recorder := &statusPatchRecorder{
		err:        statusPatchErr,
		failOnCall: failOnCall,
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(authRegistration).
		WithStatusSubresource(authRegistration).
		WithInterceptorFuncs(interceptor.Funcs{
			SubResourcePatch: recorder.interceptStatusPatch,
		}).
		Build()

	patchTarget := &authregistrationv1.AuthRegistration{}
	require.NoError(t, c.Get(context.Background(), key, patchTarget))

	return &authRegistrationStatusPatcher{Client: c}, c, patchTarget, key, recorder
}

func getAuthRegistrationFromClient(t *testing.T, c ctrlclient.Client, key types.NamespacedName) *authregistrationv1.AuthRegistration {
	t.Helper()

	got := &authregistrationv1.AuthRegistration{}
	require.NoError(t, c.Get(context.Background(), key, got))

	return got
}

type statusPatchRecorder struct {
	statusPatchCalls int
	failOnCall       int
	err              error
}

func (r *statusPatchRecorder) interceptStatusPatch(
	ctx context.Context,
	c ctrlclient.Client,
	subResourceName string,
	obj ctrlclient.Object,
	patch ctrlclient.Patch,
	opts ...ctrlclient.SubResourcePatchOption,
) error {
	if subResourceName == "status" {
		r.statusPatchCalls++
	}

	if r.err != nil && (r.failOnCall <= 0 || r.statusPatchCalls == r.failOnCall) {
		return r.err
	}

	return c.SubResource(subResourceName).Patch(ctx, obj, patch, opts...)
}
