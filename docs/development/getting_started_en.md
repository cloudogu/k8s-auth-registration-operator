# Getting Started

This repository contains the `k8s-auth-registration-operator`.

## Prerequisites

- A running Cloudogu EcoSystem Kubernetes cluster
- `kubectl` access to that cluster
- The `AuthRegistration` CRD installed in the cluster

Check if the CRD exists:

```shell
kubectl get crd authregistrations.k8s.cloudogu.com
```

## Install the Operator as a Component

From this repository:

```shell
make component-apply
```

This target builds and packages the Helm chart and applies the `Component` resource.

## Verify Installation

```shell
kubectl -n ecosystem get deployment k8s-auth-registration-operator
kubectl -n ecosystem get pods -l app.kubernetes.io/name=k8s-auth-registration-operator
```
