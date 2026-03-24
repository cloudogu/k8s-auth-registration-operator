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

The operator also needs CAS API access. For local development, ensure that the deployed chart
receives these values:

- `manager.cas.baseUrl`
- `manager.cas.authSecret.name`
- `manager.cas.authSecret.usernameKey`
- `manager.cas.authSecret.passwordKey`
- optional `manager.cas.timeout`

Example CAS settings from `k8s/helm/values.yaml`:

```yaml
manager:
  cas:
    baseUrl: "http://lop-idp-cas:8080/cas/"
    timeout: 10s
    authSecret:
      name: "lop-idp-cas-actuator-auth"
      usernameKey: username
      passwordKey: password
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

## 5. Verify a registration manually

Apply or update an `AuthRegistration` and check:

```shell
kubectl -n ecosystem get authregistrations
kubectl -n ecosystem describe authregistration <name>
kubectl -n ecosystem get secret <resolved-secret-name> -o yaml
```

For `OIDC` and `OAUTH`, the generated Secret keys follow the legacy CAS naming scheme:

- `oidc_client_id`
- `oidc_client_secret`
- `oidc_issuer_url`
- `oauth`
- `oauth_client_secret`
- `oauth_auth_url`
- `oauth_token_url`
