# Architecture Overview

## Main components

- `AuthRegistrationReconciler`
  Handles reconcile flow, finalizer handling, and orchestration.
- `serviceRegistrationBackend`
  Abstract backend interface (`Upsert`, `Delete`) for connecting the actual service registration (e.g. CAS).
- `authRegistrationSecretReconciler`
  Creates or updates credentials `Secret` objects.
- `authRegistrationStatusPatcher`
  Patches `status.resolvedSecretRef` and status conditions.

## Reconcile flow (high-level)

1. Load `AuthRegistration`.
2. If deleting: call backend delete and remove finalizer.
3. Ensure finalizer exists.
4. Upsert backend registration.
5. Resolve secret name:
   - default: `<authRegistration-name>-credentials`
   - or explicit `spec.secretRef`
6. Patch `status.resolvedSecretRef`.
7. Reconcile credentials secret.
8. Patch status conditions (`CredentialsPublished`, `Completed`).

## Secret ownership behavior

- If `spec.secretRef` is omitted:
  Controller-managed secret with owner reference + `k8s.cloudogu.com/generated-secret: "true"` annotation.
- If `spec.secretRef` is set:
  Secret is still reconciled, but without owner reference management.
