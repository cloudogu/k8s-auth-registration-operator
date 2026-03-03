# Troubleshoot a Failing AuthRegistration

## 1. Inspect status and conditions

```shell
kubectl -n ecosystem describe authregistration <name>
kubectl -n ecosystem get authregistration <name> -o yaml
```

Focus on:

- `status.resolvedSecretRef`
- `status.conditions[].type`
- `status.conditions[].reason`
- `status.conditions[].message`

## 2. Check operator logs

```shell
kubectl -n ecosystem logs deploy/k8s-auth-registration-operator --tail=200
```

## 3. Validate target secret

```shell
kubectl -n ecosystem get secret <resolved-secret-name> -o yaml
```

## 4. Common failure patterns

- `InvalidSpec`:
  `spec.secretRef` is empty or invalid after trimming.
- `SecretReconcileFailed`:
  Secret create/update failed.
- `RegistrationFailed`:
  backend upsert failed.

## 5. Retry

After fixing the cause, re-apply the resource:

```shell
kubectl apply -f auth-registration.yaml
```
