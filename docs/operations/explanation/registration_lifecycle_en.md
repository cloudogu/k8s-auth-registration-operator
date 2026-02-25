# AuthRegistration Lifecycle

## Create / Update

When an `AuthRegistration` is created or changed, the controller:

1. Converts spec into an internal registration model.
2. Calls backend `Upsert`.
3. Resolves the effective secret name.
4. Reconciles secret data (`protocol` plus backend-specific keys).
5. Updates status fields and conditions.

## Delete

On deletion, the controller uses finalizers:

1. Detects deletion timestamp.
2. Calls backend `Delete`.
3. Removes finalizer.
4. Lets Kubernetes finish resource deletion.

For controller-managed credentials secrets, owner references allow automatic secret cleanup.

## Idempotency

The secret and status reconcilers avoid unnecessary updates by comparing current and desired state before patch/update calls.
