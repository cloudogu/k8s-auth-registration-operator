package controller

import (
	"context"
	"fmt"

	authregistrationv1 "github.com/cloudogu/k8s-auth-registration-lib/api/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type authRegistrationStatusPatcher struct {
	Client client.Client
}

func (p *authRegistrationStatusPatcher) PatchInvalidSpec(ctx context.Context, authRegistration *authregistrationv1.AuthRegistration, invalidSpecErr error) error {
	if err := p.PatchResolvedSecretRef(ctx, authRegistration, ""); err != nil {
		return fmt.Errorf("failed to clear resolved secret reference: %w", err)
	}

	if err := p.patchCondition(ctx, authRegistration, metav1.Condition{
		Type:    authregistrationv1.ConditionCredentialsPublished,
		Status:  metav1.ConditionFalse,
		Reason:  "InvalidSpec",
		Message: invalidSpecErr.Error(),
	}); err != nil {
		return err
	}

	return p.patchCondition(ctx, authRegistration, metav1.Condition{
		Type:    authregistrationv1.ConditionCompleted,
		Status:  metav1.ConditionFalse,
		Reason:  "InvalidSpec",
		Message: "Registration is blocked because the resource specification is invalid",
	})
}

func (p *authRegistrationStatusPatcher) PatchSecretReconcileFailed(ctx context.Context, authRegistration *authregistrationv1.AuthRegistration, resolvedSecretRef string, secretErr error) error {
	if err := p.patchCondition(ctx, authRegistration, metav1.Condition{
		Type:    authregistrationv1.ConditionCredentialsPublished,
		Status:  metav1.ConditionFalse,
		Reason:  "SecretReconcileFailed",
		Message: fmt.Sprintf("Failed to reconcile Secret %q: %v", resolvedSecretRef, secretErr),
	}); err != nil {
		return err
	}

	return p.patchCondition(ctx, authRegistration, metav1.Condition{
		Type:    authregistrationv1.ConditionCompleted,
		Status:  metav1.ConditionFalse,
		Reason:  "Blocked",
		Message: "Registration is blocked until Secret reconciliation succeeds",
	})
}

func (p *authRegistrationStatusPatcher) PatchCredentialsPublished(ctx context.Context, authRegistration *authregistrationv1.AuthRegistration) error {
	return p.patchCondition(ctx, authRegistration, metav1.Condition{
		Type:    authregistrationv1.ConditionCredentialsPublished,
		Status:  metav1.ConditionTrue,
		Reason:  "SecretReconciled",
		Message: "Credentials Secret is ready",
	})
}

func (p *authRegistrationStatusPatcher) PatchRegistrationFailed(ctx context.Context, authRegistration *authregistrationv1.AuthRegistration, registrationErr error) error {
	return p.patchCondition(ctx, authRegistration, metav1.Condition{
		Type:    authregistrationv1.ConditionCompleted,
		Status:  metav1.ConditionFalse,
		Reason:  "RegistrationFailed",
		Message: fmt.Sprintf("Failed to register service in backend: %v", registrationErr),
	})
}

func (p *authRegistrationStatusPatcher) PatchRegistrationSucceeded(ctx context.Context, authRegistration *authregistrationv1.AuthRegistration) error {
	return p.patchCondition(ctx, authRegistration, metav1.Condition{
		Type:    authregistrationv1.ConditionCompleted,
		Status:  metav1.ConditionTrue,
		Reason:  "RegistrationSucceeded",
		Message: "Service registration was reconciled successfully",
	})
}

func (p *authRegistrationStatusPatcher) PatchResolvedSecretRef(ctx context.Context, authRegistration *authregistrationv1.AuthRegistration, resolvedSecretRef string) error {
	return p.patchStatus(ctx, authRegistration, func(status *authregistrationv1.AuthRegistrationStatus) {
		status.ResolvedSecretRef = resolvedSecretRef
	})
}

func (p *authRegistrationStatusPatcher) patchCondition(ctx context.Context, authRegistration *authregistrationv1.AuthRegistration, condition metav1.Condition) error {
	if condition.Type == "" {
		return fmt.Errorf("condition type must not be empty")
	}

	return p.patchStatus(ctx, authRegistration, func(status *authregistrationv1.AuthRegistrationStatus) {
		condition.ObservedGeneration = authRegistration.Generation
		meta.SetStatusCondition(&status.Conditions, condition)
	})
}

func (p *authRegistrationStatusPatcher) patchStatus(ctx context.Context, authRegistration *authregistrationv1.AuthRegistration, mutate func(status *authregistrationv1.AuthRegistrationStatus)) error {
	authRegistrationBeforePatch := authRegistration.DeepCopy()
	mutate(&authRegistration.Status)

	if equality.Semantic.DeepEqual(authRegistrationBeforePatch.Status, authRegistration.Status) {
		return nil
	}

	return p.Client.Status().Patch(ctx, authRegistration, client.MergeFrom(authRegistrationBeforePatch))
}
