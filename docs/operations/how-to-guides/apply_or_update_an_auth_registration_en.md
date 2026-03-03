# How to Apply or Update an AuthRegistration

Create an `AuthRegistration` resource and apply it with `kubectl`.

## Example

```yaml
apiVersion: k8s.cloudogu.com/v1
kind: AuthRegistration
metadata:
  name: my-service
  namespace: ecosystem
spec:
  protocol: OIDC
  consumer: my-service
  logoutURL: https://my-service.example/logout
  params:
    tenant: default
```

Apply or update:

```shell
kubectl apply -f auth-registration.yaml
```

## Verify reconciliation

```shell
kubectl -n ecosystem get authregistration my-service -o yaml
kubectl -n ecosystem get secret my-service-credentials -o yaml
```

If `spec.secretRef` is set, use that secret name instead of `my-service-credentials`.
