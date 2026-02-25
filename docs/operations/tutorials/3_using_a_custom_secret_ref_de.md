# Benutzerdefiniertes SecretRef verwenden

Über `spec.secretRef` kann ein eigener Ziel-Secret-Name gesetzt werden.

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

Anwenden:

```shell
kubectl apply -f demo-auth-registration-custom-secret.yaml
```

Status und Secret prüfen:

```shell
kubectl -n ecosystem get authregistration demo-app-custom-secret -o yaml
kubectl -n ecosystem get secret demo-app-oidc-credentials -o yaml
```

Hinweise:

- `status.resolvedSecretRef` zeigt auf `demo-app-oidc-credentials`.
- Leere Werte wie `"   "` für `secretRef` sind ungültig.
