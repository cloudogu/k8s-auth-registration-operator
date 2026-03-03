# Operator-Konfiguration

Der Operator kann über Helm-Values, Umgebungsvariablen und Runtime-Flags konfiguriert werden.

## Helm-Values (`k8s/helm/values.yaml`)

| Parameter                         | Beschreibung                                 | Default                                   |
|:----------------------------------|:---------------------------------------------|:------------------------------------------|
| `global.imagePullSecrets`         | Pull-Secrets für Container-Images            | `[{name: "ces-container-registries"}]`    |
| `manager.replicas`                | Anzahl der Operator-Pods                     | `1`                                       |
| `manager.image.registry`          | Image-Registry                               | `docker.io`                               |
| `manager.image.repository`        | Image-Repository                             | `cloudogu/k8s-auth-registration-operator` |
| `manager.image.tag`               | Image-Tag                                    | `0.0.1`                                   |
| `manager.imagePullPolicy`         | Kubernetes Image-Pull-Policy                 | `IfNotPresent`                            |
| `manager.env.logLevel`            | Log-Level (`debug`, `info`, `warn`, `error`) | `info`                                    |
| `manager.env.stage`               | Stage (`development`, `production`)          | `production`                              |
| `manager.resourceLimits.memory`   | Container-Memory-Limit                       | `128M`                                    |
| `manager.resourceRequests.cpu`    | CPU-Request                                  | `50m`                                     |
| `manager.resourceRequests.memory` | Memory-Request                               | `128M`                                    |
| `manager.networkPolicies.enabled` | Deny-All-Ingress-Policy erzeugen             | `true`                                    |
