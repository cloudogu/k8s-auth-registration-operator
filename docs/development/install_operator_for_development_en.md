# Install the Operator for Development

This guide shows a practical local workflow for developing the operator.

## 1. Configure local environment

Create or adjust `.env` in the repository root, for example:

```dotenv
NAMESPACE=ecosystem
STAGE=development
LOG_LEVEL=debug
RUNTIME_ENV=local
KUBE_CONTEXT_NAME=k3ces.local
```

## 2. Build and deploy development artifact

```shell
make build-boot
```

`build-boot` applies the chart and restarts the operator pod.

## 3. Inspect logs

```shell
kubectl -n ecosystem logs -f deploy/k8s-auth-registration-operator
```

## 4. Run unit tests

```shell
go test ./...
```
