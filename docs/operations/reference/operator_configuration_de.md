# Operator-Konfiguration

Der Operator kann über Helm-Values, Umgebungsvariablen und Runtime-Flags konfiguriert werden.

## Helm-Values (`k8s/helm/values.yaml`)

| Parameter                            | Beschreibung                                 | Default                                   |
|:-------------------------------------|:---------------------------------------------|:------------------------------------------|
| `global.imagePullSecrets`            | Pull-Secrets für Container-Images            | `[{name: "ces-container-registries"}]`    |
| `manager.replicas`                   | Anzahl der Operator-Pods                     | `1`                                       |
| `manager.image.registry`             | Image-Registry                               | `docker.io`                               |
| `manager.image.repository`           | Image-Repository                             | `cloudogu/k8s-auth-registration-operator` |
| `manager.image.tag`                  | Image-Tag                                    | `0.0.1`                                   |
| `manager.imagePullPolicy`            | Kubernetes Image-Pull-Policy                 | `IfNotPresent`                            |
| `manager.env.logLevel`               | Log-Level (`debug`, `info`, `warn`, `error`) | `info`                                    |
| `manager.env.stage`                  | Stage (`development`, `production`)          | `production`                              |
| `manager.cas.baseUrl`                | CAS-Base-URL inkl. `/cas`-Kontext            | `""`                                      |
| `manager.cas.timeout`                | HTTP-Timeout für CAS-API-Aufrufe             | `10s`                                     |
| `manager.cas.authSecret.name`        | Secret mit CAS-Basic-Auth-Credentials        | `""`                                      |
| `manager.cas.authSecret.usernameKey` | Secret-Key für den CAS-Benutzernamen         | `username`                                |
| `manager.cas.authSecret.passwordKey` | Secret-Key für das CAS-Passwort              | `password`                                |
| `manager.resourceLimits.memory`      | Container-Memory-Limit                       | `128M`                                    |
| `manager.resourceRequests.cpu`       | CPU-Request                                  | `50m`                                     |
| `manager.resourceRequests.memory`    | Memory-Request                               | `128M`                                    |
| `manager.networkPolicies.enabled`    | Deny-All-Ingress-Policy erzeugen             | `true`                                    |

## Umgebungsvariablen

Das Deployment rendert diese CAS-bezogenen Umgebungsvariablen in den Operator-Pod:

| Umgebungsvariable | Beschreibung                                 |
|:------------------|:---------------------------------------------|
| `CAS_BASE_URL`    | CAS-Base-URL inklusive `/cas`-Kontextpfad    |
| `CAS_USERNAME`    | Benutzername für CAS-Actuator-Basic-Auth     |
| `CAS_PASSWORD`    | Passwort für CAS-Actuator-Basic-Auth         |
| `CAS_TIMEOUT`     | Optionaler HTTP-Timeout für CAS-API-Requests |

Der Operator startet nicht, wenn `CAS_BASE_URL`, `CAS_USERNAME` oder `CAS_PASSWORD` fehlen oder ungültig sind.

## Erzeugte Secret-Keys

Für erfolgreiche Registrierungen schreibt der Operator protokollspezifische Werte in den aufgelösten Secret.
Die Key-Namen folgen dem Legacy-CAS-Schema, das von bestehenden Dogus erwartet wird.

| Protokoll | Secret-Keys                                                         |
|:----------|:--------------------------------------------------------------------|
| `CAS`     | `cas_client_id`                                                     |
| `OIDC`    | `oidc_client_id`, `oidc_client_secret`, `oidc_issuer_url`           |
| `OAUTH`   | `oauth`, `oauth_client_secret`, `oauth_auth_url`, `oauth_token_url` |
