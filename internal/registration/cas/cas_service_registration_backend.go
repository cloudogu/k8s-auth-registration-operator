package cas

import (
	"context"

	"github.com/cloudogu/k8s-auth-registration-operator/internal/domain"
)

type CASServiceRegistrationBackend struct{}

func NewCASServiceRegistrationBackend() (*CASServiceRegistrationBackend, error) {
	return &CASServiceRegistrationBackend{}, nil
}

func (b *CASServiceRegistrationBackend) Upsert(_ context.Context, _ domain.Registration) (domain.RegistrationResult, error) {
	panic("not implemented")

	return domain.RegistrationResult{}, nil
}

func (b *CASServiceRegistrationBackend) Delete(_ context.Context, _ domain.Registration) error {
	panic("not implemented")

	return nil
}
