package controller

import (
	"context"
	"fmt"
	"maps"

	authregistrationv1 "github.com/cloudogu/k8s-auth-registration-lib/api/v1"
	"github.com/cloudogu/k8s-auth-registration-operator/internal/domain"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	authRegistrationNameLabelKey = "k8s.cloudogu.com/auth-registration"
)

type authRegistrationSecretReconciler struct {
	Client client.Client
	Scheme *runtime.Scheme
}

func (r *authRegistrationSecretReconciler) Reconcile(
	ctx context.Context,
	registrationResult domain.RegistrationResult,
	authRegistration *authregistrationv1.AuthRegistration,
	secretName string,
	isControllerManagedSecret bool,
) error {
	desiredSecret := buildDesiredSecret(registrationResult, authRegistration, secretName)
	secretObjectKey := types.NamespacedName{Name: secretName, Namespace: authRegistration.Namespace}

	var currentSecret corev1.Secret
	err := r.Client.Get(ctx, secretObjectKey, &currentSecret)
	if apierrors.IsNotFound(err) {
		// if the secret does not exist, set the owner reference before creating it
		// this is to avoid orphaned secrets that we created
		if ownerReferenceErr := controllerutil.SetControllerReference(authRegistration, desiredSecret, r.Scheme); ownerReferenceErr != nil {
			return ownerReferenceErr
		}

		return r.Client.Create(ctx, desiredSecret)
	}
	if err != nil {
		return fmt.Errorf("failed to get secret %q: %w", secretName, err)
	}

	secretBeforeUpdate := currentSecret.DeepCopy()

	currentSecret.Type = desiredSecret.Type

	if currentSecret.Labels == nil {
		currentSecret.Labels = map[string]string{}
	}
	maps.Copy(currentSecret.Labels, desiredSecret.Labels)

	if currentSecret.Annotations == nil {
		currentSecret.Annotations = map[string]string{}
	}
	maps.Copy(currentSecret.Annotations, desiredSecret.Annotations)

	if currentSecret.Data == nil {
		currentSecret.Data = map[string][]byte{}
	}
	maps.Copy(currentSecret.Data, desiredSecret.Data)

	if isControllerManagedSecret {
		if ownerReferenceErr := controllerutil.SetControllerReference(authRegistration, &currentSecret, r.Scheme); ownerReferenceErr != nil {
			return ownerReferenceErr
		}
	}

	if equality.Semantic.DeepEqual(secretBeforeUpdate, &currentSecret) {
		return nil
	}

	return r.Client.Update(ctx, &currentSecret)
}

func buildDesiredSecret(result domain.RegistrationResult, authRegistration *authregistrationv1.AuthRegistration, secretName string) *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: authRegistration.Namespace,
			Labels: map[string]string{
				authRegistrationNameLabelKey: authRegistration.Name,
			},
			Annotations: map[string]string{},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{},
	}

	// add secret data from registration-result
	maps.Copy(secret.Data, result.GetSecretData())

	return secret
}
