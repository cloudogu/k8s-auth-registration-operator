# Erste Schritte

Dieses Repository enthält den `k8s-auth-registration-operator`.

## Voraussetzungen

- Ein laufendes Cloudogu EcoSystem Kubernetes-Cluster
- `kubectl`-Zugriff auf das Cluster
- Die `AuthRegistration`-CRD ist im Cluster installiert

Prüfen, ob die CRD vorhanden ist:

```shell
kubectl get crd authregistrations.k8s.cloudogu.com
```

## Operator als Component installieren

Aus diesem Repository:

```shell
make component-apply
```

Dieses Target baut und paketiert das Helm-Chart und wendet die `Component`-Ressource an.

## Installation verifizieren

```shell
kubectl -n ecosystem get deployment k8s-auth-registration-operator
kubectl -n ecosystem get pods -l app.kubernetes.io/name=k8s-auth-registration-operator
```
