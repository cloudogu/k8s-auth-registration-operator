# Installing the Operator

Install `k8s-auth-registration-operator` as a component in your EcoSystem namespace.

```yaml
apiVersion: k8s.cloudogu.com/v1
kind: Component
metadata:
  name: k8s-auth-registration-operator
spec:
  name: k8s-auth-registration-operator
  namespace: k8s
  # version: <desired-version>
```

Apply:

```shell
kubectl -n ecosystem apply -f component-auth-registration-operator.yaml
```

Verify:

```shell
kubectl -n ecosystem get deploy k8s-auth-registration-operator
kubectl -n ecosystem get pods -l app.kubernetes.io/name=k8s-auth-registration-operator
```
