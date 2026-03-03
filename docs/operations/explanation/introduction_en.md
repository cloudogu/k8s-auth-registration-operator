# Introduction

The `k8s-auth-registration-operator` reconciles `AuthRegistration` custom resources and implements declarative SSO
registration in Kubernetes. Instead of creating registrations manually in downstream systems, the desired state is
defined as a Kubernetes resource and continuously reconciled by the controller.

For each `AuthRegistration`, the operator executes backend registration, resolves the effective credentials secret,
writes the resulting secret data, and then patches status fields and conditions. This keeps protocol, consumer, and
additional parameters transparent in cluster configuration while credentials publication and failure handling stay
reproducible and controller-driven.

