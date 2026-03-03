# AuthRegistration-Lebenszyklus

## Create / Update

Wenn eine `AuthRegistration` erstellt oder geändert wird, macht der Controller:

1. Konvertiert die Spec in ein internes Registration-Modell.
2. Ruft Backend-`Upsert` auf.
3. Löst den effektiven Secret-Namen auf.
4. Reconciled Secret-Daten (`protocol` plus backend-spezifische Keys).
5. Aktualisiert Status-Felder und Conditions.

## Delete

Bei Löschung verwendet der Controller Finalizer:

1. Deletion-Timestamp erkennen.
2. Backend-`Delete` aufrufen.
3. Finalizer entfernen.
4. Kubernetes die finale Löschung ausführen lassen.

Bei Controller-verwalteten Credentials-Secrets sorgt die OwnerReference für automatisches Aufräumen.

## Idempotenz

Secret- und Status-Reconciler vermeiden unnötige Updates, indem sie aktuellen und gewünschten Zustand vor Patch/Update vergleichen.
