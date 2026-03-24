# Operator Configuration

The operator can be configured via Helm values, environment variables, and runtime flags.

## Helm values (`k8s/helm/values.yaml`)

| Parameter                            | Description                                  | Default                                   |
|:-------------------------------------|:---------------------------------------------|:------------------------------------------|
| `global.imagePullSecrets`            | Pull secrets for container images            | `[{name: "ces-container-registries"}]`    |
| `manager.replicas`                   | Number of operator pods                      | `1`                                       |
| `manager.image.registry`             | Image registry                               | `docker.io`                               |
| `manager.image.repository`           | Image repository                             | `cloudogu/k8s-auth-registration-operator` |
| `manager.image.tag`                  | Image tag                                    | `0.0.1`                                   |
| `manager.imagePullPolicy`            | Kubernetes image pull policy                 | `IfNotPresent`                            |
| `manager.env.logLevel`               | Log level (`debug`, `info`, `warn`, `error`) | `info`                                    |
| `manager.env.stage`                  | Stage (`development`, `production`)          | `production`                              |
| `manager.cas.baseUrl`                | CAS base URL incl. `/cas` context            | `""`                                      |
| `manager.cas.timeout`                | HTTP timeout for CAS API calls               | `10s`                                     |
| `manager.cas.authSecret.name`        | Secret containing CAS basic-auth credentials | `""`                                      |
| `manager.cas.authSecret.usernameKey` | Secret key for the CAS username              | `username`                                |
| `manager.cas.authSecret.passwordKey` | Secret key for the CAS password              | `password`                                |
| `manager.resourceLimits.memory`      | Container memory limit                       | `128M`                                    |
| `manager.resourceRequests.cpu`       | CPU request                                  | `50m`                                     |
| `manager.resourceRequests.memory`    | Memory request                               | `128M`                                    |
| `manager.networkPolicies.enabled`    | Create deny-all ingress policy               | `true`                                    |

## Environment variables

The deployment renders these CAS-related environment variables into the operator pod:

| Environment variable | Description                                    |
|:---------------------|:-----------------------------------------------|
| `CAS_BASE_URL`       | CAS base URL including the `/cas` context path |
| `CAS_USERNAME`       | Username for CAS actuator basic-auth           |
| `CAS_PASSWORD`       | Password for CAS actuator basic-auth           |
| `CAS_TIMEOUT`        | Optional HTTP timeout for CAS API requests     |

The operator fails to start when `CAS_BASE_URL`, `CAS_USERNAME`, or `CAS_PASSWORD` are missing or invalid.

## Generated Secret keys

For successful registrations, the operator writes protocol-specific values into the resolved Secret.
The key names follow the legacy CAS naming scheme expected by existing Dogus.

| Protocol | Secret keys                                                         |
|:---------|:--------------------------------------------------------------------|
| `CAS`    | `cas_client_id`                                                     |
| `OIDC`   | `oidc_client_id`, `oidc_client_secret`, `oidc_issuer_url`           |
| `OAUTH`  | `oauth`, `oauth_client_secret`, `oauth_auth_url`, `oauth_token_url` |
