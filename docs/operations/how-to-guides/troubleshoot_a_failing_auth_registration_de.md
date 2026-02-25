# Fehlgeschlagene AuthRegistration analysieren

## 1. Status und Conditions prüfen

```shell
kubectl -n ecosystem describe authregistration <name>
kubectl -n ecosystem get authregistration <name> -o yaml
```

Wichtige Felder:

- `status.resolvedSecretRef`
- `status.conditions[].type`
- `status.conditions[].reason`
- `status.conditions[].message`

## 2. Operator-Logs prüfen

```shell
kubectl -n ecosystem logs deploy/k8s-auth-registration-operator --tail=200
```

## 3. Ziel-Secret validieren

```shell
kubectl -n ecosystem get secret <resolved-secret-name> -o yaml
```

## 4. Typische Fehlerbilder

- `InvalidSpec`:
  `spec.secretRef` ist leer oder nach Trimming ungültig.
- `SecretReconcileFailed`:
  Secret-Erstellung oder -Update ist fehlgeschlagen.
- `RegistrationFailed`:
  Backend-Upsert ist fehlgeschlagen.

## 5. Erneut auslösen

Nach der Korrektur Ressource erneut anwenden:

```shell
kubectl apply -f auth-registration.yaml
```
