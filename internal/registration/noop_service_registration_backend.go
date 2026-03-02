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
	result := domain.RegistrationResult{
		Protocol:       reg.Protocol,
		RegistrationID: fmt.Sprintf("%s-%s", reg.Protocol, reg.Consumer),
	}

	switch reg.Protocol {
	case domain.ProtocolOIDC:
		result.OIDC = &domain.OIDCResult{
			ClientID:     reg.Consumer,
			ClientSecret: "verySecretClientSecret",
			IssuerURL:    "https://issuer.example.com",
		}
	case domain.ProtocolOAuth:
		result.OAuth = &domain.OAuthResult{
			ClientID:     reg.Consumer,
			ClientSecret: "verySecretClientSecret",
			AuthURL:      "https://auth.example.com",
			TokenURL:     "https://token.example.com",
		}
	case domain.ProtocolCAS:
		result.CAS = &domain.CASResult{
			ServiceID: fmt.Sprintf("service-%s", reg.Consumer),
		}
	default:
		return domain.RegistrationResult{}, fmt.Errorf("unsupported protocol %q", reg.Protocol)
	}

	return result, nil
}

func (b *NoOpServiceRegistrationBackend) Delete(_ context.Context, _ domain.Registration) error {
	return nil
}
