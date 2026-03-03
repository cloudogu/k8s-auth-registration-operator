# AuthRegistration anwenden oder aktualisieren

Erstelle eine `AuthRegistration`-Ressource und wende sie mit `kubectl` an.

## Beispiel

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

Anwenden oder aktualisieren:

```shell
kubectl apply -f auth-registration.yaml
```

## Reconciliation prüfen

```shell
kubectl -n ecosystem get authregistration my-service -o yaml
kubectl -n ecosystem get secret my-service-credentials -o yaml
```

Wenn `spec.secretRef` gesetzt ist, nutze statt `my-service-credentials` den dort angegebenen Secret-Namen.
