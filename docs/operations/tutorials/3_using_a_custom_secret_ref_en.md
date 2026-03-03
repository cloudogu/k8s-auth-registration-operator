# Using a Custom SecretRef

You can provide a custom target secret name via `spec.secretRef`.

```yaml
apiVersion: k8s.cloudogu.com/v1
kind: AuthRegistration
metadata:
  name: demo-app-custom-secret
  namespace: ecosystem
spec:
  protocol: OIDC
  consumer: demo-app
  secretRef: demo-app-oidc-credentials
```

Apply:

```shell
kubectl apply -f demo-auth-registration-custom-secret.yaml
```

Verify status and secret:

```shell
kubectl -n ecosystem get authregistration demo-app-custom-secret -o yaml
kubectl -n ecosystem get secret demo-app-oidc-credentials -o yaml
```

Notes:

- `status.resolvedSecretRef` points to `demo-app-oidc-credentials`.
- Empty values like `"   "` for `secretRef` are invalid.
