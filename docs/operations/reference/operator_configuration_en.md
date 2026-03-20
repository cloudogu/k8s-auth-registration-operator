# Operator Configuration

The operator can be configured via Helm values, environment variables, and runtime flags.

## Helm values (`k8s/helm/values.yaml`)

| Parameter                         | Description                                  | Default                                   |
|:----------------------------------|:---------------------------------------------|:------------------------------------------|
| `global.imagePullSecrets`         | Pull secrets for container images            | `[{name: "ces-container-registries"}]`    |
| `manager.replicas`                | Number of operator pods                      | `1`                                       |
| `manager.image.registry`          | Image registry                               | `docker.io`                               |
| `manager.image.repository`        | Image repository                             | `cloudogu/k8s-auth-registration-operator` |
| `manager.image.tag`               | Image tag                                    | `0.0.1`                                   |
| `manager.imagePullPolicy`         | Kubernetes image pull policy                 | `IfNotPresent`                            |
| `manager.env.logLevel`            | Log level (`debug`, `info`, `warn`, `error`) | `info`                                    |
| `manager.env.stage`               | Stage (`development`, `production`)          | `production`                              |
| `manager.cas.baseUrl`             | CAS base URL incl. `/cas` context            | `""`                                      |
| `manager.cas.timeout`             | HTTP timeout for CAS API calls               | `10s`                                     |
| `manager.cas.existingSecret.name` | Secret containing CAS basic-auth credentials | `""`                                      |
| `manager.cas.existingSecret.usernameKey` | Secret key for the CAS username      | `username`                                |
| `manager.cas.existingSecret.passwordKey` | Secret key for the CAS password      | `password`                                |
| `manager.resourceLimits.memory`   | Container memory limit                       | `128M`                                    |
| `manager.resourceRequests.cpu`    | CPU request                                  | `50m`                                     |
| `manager.resourceRequests.memory` | Memory request                               | `128M`                                    |
| `manager.networkPolicies.enabled` | Create deny-all ingress policy               | `true`                                    |
