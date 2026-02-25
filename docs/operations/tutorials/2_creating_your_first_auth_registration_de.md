# Erste AuthRegistration erstellen

Erstelle ein erstes OIDC-Registrierungsobjekt.

```yaml
apiVersion: k8s.cloudogu.com/v1
kind: AuthRegistration
metadata:
  name: demo-app
  namespace: ecosystem
spec:
  protocol: OIDC
  consumer: demo-app
```

Anwenden:

```shell
kubectl apply -f demo-auth-registration.yaml
```

Status prüfen:

```shell
kubectl -n ecosystem get authregistration demo-app -o yaml
```

Generiertes Credentials-Secret prüfen:

```shell
kubectl -n ecosystem get secret demo-app-credentials -o yaml
```

Das Standard-Schema für den Secret-Namen ist `<name>-credentials`.

Hinweis:

- Das aktuell standardmäßig verwendete Backend ist `NoOpServiceRegistrationBackend`.
- Die erzeugten Credentials sind Platzhalterwerte für die Entwicklung.
