# Operator installieren

Installiere den `k8s-auth-registration-operator` als Component im EcoSystem-Namespace.

```yaml
apiVersion: k8s.cloudogu.com/v1
kind: Component
metadata:
  name: k8s-auth-registration-operator
spec:
  name: k8s-auth-registration-operator
  namespace: k8s
  # version: <gewünschte-version>
```

Anwenden:

```shell
kubectl -n ecosystem apply -f component-auth-registration-operator.yaml
```

Prüfen:

```shell
kubectl -n ecosystem get deploy k8s-auth-registration-operator
kubectl -n ecosystem get pods -l app.kubernetes.io/name=k8s-auth-registration-operator
```
