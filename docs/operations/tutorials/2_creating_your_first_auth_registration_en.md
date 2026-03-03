# Creating Your First AuthRegistration

Create a first OIDC registration object.

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

Apply:

```shell
kubectl apply -f demo-auth-registration.yaml
```

Check status:

```shell
kubectl -n ecosystem get authregistration demo-app -o yaml
```

Check generated credentials secret:

```shell
kubectl -n ecosystem get secret demo-app-credentials -o yaml
```

The default secret name pattern is `<name>-credentials`.

Note:

- The current default backend is `NoOpServiceRegistrationBackend`.
- The resulting credentials are placeholder values for development.
