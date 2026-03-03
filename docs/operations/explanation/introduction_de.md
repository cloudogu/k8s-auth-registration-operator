# Einführung

Der `k8s-auth-registration-operator` reconciled `AuthRegistration`-Custom-Resources und übernimmt die technische
Umsetzung einer deklarativen SSO-Registrierung in Kubernetes. Statt Registrierungen manuell in nachgelagerten Systemen
anzulegen, wird der gewünschte Zustand als Ressource beschrieben und vom Controller kontinuierlich in den Ist-Zustand
überführt.

Für jede `AuthRegistration`-Ressource führt der Operator im Reconcile-Ablauf die Registrierung im Backend aus, ermittelt
das Ziel-Secret für Credentials, schreibt die resultierenden Secret-Daten und aktualisiert anschließend den Status
inklusive Conditions. Dadurch sind Protokoll, Consumer und weitere Parameter transparent in der Cluster-Konfiguration
beschrieben, während Veröffentlichung der Credentials und Fehlerbehandlung reproduzierbar durch den Controller erfolgen.

