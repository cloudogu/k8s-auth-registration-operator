# Architekturüberblick

## Hauptkomponenten

- `AuthRegistrationReconciler`
  Steuert den Reconcile-Flow, Finalizer-Handling und Orchestrierung.
- `serviceRegistrationBackend`
  Abstraktes Backend-Interface (`Upsert`, `Delete`) zur Anbindung der eigentlichen ServiceRegistration (z.B. CAS).
- `authRegistrationSecretReconciler`
  Erstellt oder aktualisiert Credentials-`Secret`-Objekte.
- `authRegistrationStatusPatcher`
  Patcht `status.resolvedSecretRef` und Status-Conditions.

## Reconcile-Flow (High-Level)

1. `AuthRegistration` laden.
2. Bei Löschung: Backend-Delete aufrufen und Finalizer entfernen.
3. Sicherstellen, dass der Finalizer vorhanden ist.
4. Backend-Registrierung upserten.
5. Secret-Namen auflösen:
   - Standard: `<authRegistration-name>-credentials`
   - oder explizites `spec.secretRef`
6. `status.resolvedSecretRef` patchen.
7. Credentials-Secret reconcilen.
8. Status-Conditions patchen (`CredentialsPublished`, `Completed`).

## Secret-Ownership-Verhalten

- Wenn `spec.secretRef` fehlt:
  Controller-verwaltetes Secret mit OwnerReference + Annotation `k8s.cloudogu.com/generated-secret: "true"`.
- Wenn `spec.secretRef` gesetzt ist:
  Secret wird trotzdem reconciled, aber ohne OwnerReference-Verwaltung.
