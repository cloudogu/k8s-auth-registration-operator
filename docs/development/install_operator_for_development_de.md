# Operator für die Entwicklung installieren

Diese Anleitung zeigt einen praktikablen lokalen Workflow für die Entwicklung des Operators.

## 1. Lokale Umgebung konfigurieren

`.env` im Repository-Root anlegen oder anpassen, zum Beispiel:

```dotenv
NAMESPACE=ecosystem
STAGE=development
LOG_LEVEL=debug
RUNTIME_ENV=local
KUBE_CONTEXT_NAME=k3ces.local
```

Der Operator benötigt außerdem Zugriff auf die CAS-API. Für die lokale Entwicklung muss das
deployte Chart diese Werte erhalten:

- `manager.cas.baseUrl`
- `manager.cas.authSecret.name`
- `manager.cas.authSecret.usernameKey`
- `manager.cas.authSecret.passwordKey`
- optional `manager.cas.timeout`

Beispiel aus `k8s/helm/values.yaml`:

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

## 2. Entwicklungsartefakt bauen und deployen

```shell
make build-boot
```

`build-boot` wendet das Chart an und startet den Operator-Pod neu.

## 3. Logs prüfen

```shell
kubectl -n ecosystem logs -f deploy/k8s-auth-registration-operator
```

## 4. Unit-Tests ausführen

```shell
go test ./...
```

## 5. Registrierung manuell verifizieren

Eine `AuthRegistration` anwenden oder aktualisieren und danach prüfen:

```shell
kubectl -n ecosystem get authregistrations
kubectl -n ecosystem describe authregistration <name>
kubectl -n ecosystem get secret <resolved-secret-name> -o yaml
```

Für `OIDC` und `OAUTH` verwendet der erzeugte Secret die Legacy-CAS-Key-Namen:

- `oidc_client_id`
- `oidc_client_secret`
- `oidc_issuer_url`
- `oauth`
- `oauth_client_secret`
- `oauth_auth_url`
- `oauth_token_url`
