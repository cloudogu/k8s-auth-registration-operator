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