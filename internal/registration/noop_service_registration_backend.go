package registration

import (
	"context"
	"fmt"

	"github.com/cloudogu/k8s-auth-registration-operator/internal/domain"
)

// NoOpServiceRegistrationBackend keeps reconciliation functional while no real
// backend is configured. It can be replaced by CAS or another implementation.
type NoOpServiceRegistrationBackend struct{}

func (b *NoOpServiceRegistrationBackend) Upsert(_ context.Context, reg domain.Registration) (domain.RegistrationResult, error) {
	return domain.RegistrationResult{
		Protocol:       reg.Protocol,
		RegistrationID: fmt.Sprintf("%s-%s", reg.Protocol, reg.Consumer),
		OIDC:           &domain.OIDCResult{ClientID: reg.Consumer, ClientSecret: "verySecretClientSecret"},
	}, nil
}

func (b *NoOpServiceRegistrationBackend) Delete(_ context.Context, _ domain.Registration) error {
	return nil
}
