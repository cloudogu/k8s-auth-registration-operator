package cas

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"github.com/cloudogu/k8s-auth-registration-operator/internal/domain"
	libconfig "github.com/cloudogu/k8s-registry-lib/config"
)

var randomRead = rand.Read

type globalConfigRepo interface {
	Get(context.Context) (libconfig.GlobalConfig, error)
}

type apiClient interface {
	BaseURL() string
	ListServices(ctx context.Context) ([]RegisteredService, error)
	CreateService(ctx context.Context, service RegisteredService) (RegisteredService, error)
	UpdateService(ctx context.Context, service RegisteredService) (RegisteredService, error)
	DeleteService(ctx context.Context, id int64) error
}

type registrationMapper interface {
	ValidateRegistration(reg domain.Registration) error
	BuildServicePayload(ctx context.Context, reg domain.Registration, existing *RegisteredService, services []RegisteredService, clientSecret string) (RegisteredService, error)
	BuildRegistrationResult(reg domain.Registration, service RegisteredService, clientSecret string) (domain.RegistrationResult, error)
}

type CASServiceRegistrationBackend struct {
	client apiClient
	mapper registrationMapper
}

func NewCASServiceRegistrationBackend(globalConfigRepo globalConfigRepo, client apiClient) (*CASServiceRegistrationBackend, error) {
	if client == nil {
		return nil, fmt.Errorf("client must not be nil")
	}

	casBaseUrl, err := url.ParseRequestURI(client.BaseURL())
	if err != nil {
		return nil, fmt.Errorf("invalid base URL %q: %w", client.BaseURL(), err)
	}

	return &CASServiceRegistrationBackend{
		client: client,
		mapper: newServiceMapper(globalConfigRepo, casBaseUrl.Path),
	}, nil
}

func (b *CASServiceRegistrationBackend) Upsert(ctx context.Context, reg domain.Registration, existingData domain.RegistrationData) (domain.RegistrationResult, error) {
	if err := b.mapper.ValidateRegistration(reg); err != nil {
		return domain.RegistrationResult{}, err
	}

	services, err := b.client.ListServices(ctx)
	if err != nil {
		return domain.RegistrationResult{}, fmt.Errorf("failed to list Cas services: %w", err)
	}

	existing, err := findServiceByConsumer(services, reg.Consumer)
	if err != nil {
		return domain.RegistrationResult{}, err
	}

	clientSecret, err := b.resolveClientSecret(reg, existingData)
	if err != nil {
		return domain.RegistrationResult{}, err
	}

	service, err := b.mapper.BuildServicePayload(ctx, reg, existing, services, clientSecret)
	if err != nil {
		return domain.RegistrationResult{}, err
	}

	var persisted RegisteredService
	if existing == nil {
		persisted, err = b.client.CreateService(ctx, service)
		if err != nil {
			return domain.RegistrationResult{}, fmt.Errorf("failed to create Cas service for consumer %q: %w", reg.Consumer, err)
		}
	} else {
		persisted, err = b.client.UpdateService(ctx, service)
		if err != nil {
			return domain.RegistrationResult{}, fmt.Errorf("failed to update Cas service for consumer %q: %w", reg.Consumer, err)
		}
	}

	return b.mapper.BuildRegistrationResult(reg, persisted, clientSecret)
}

func (b *CASServiceRegistrationBackend) Delete(ctx context.Context, reg domain.Registration) error {
	if strings.TrimSpace(reg.Consumer) == "" {
		return fmt.Errorf("consumer must not be empty")
	}

	services, err := b.client.ListServices(ctx)
	if err != nil {
		return fmt.Errorf("failed to list Cas services: %w", err)
	}

	existing, err := findServiceByConsumer(services, reg.Consumer)
	if err != nil {
		return err
	}

	if existing == nil {
		return nil
	}

	if existing.ID == 0 {
		return fmt.Errorf("cas-service for consumer %q has no numeric id", reg.Consumer)
	}

	if err := b.client.DeleteService(ctx, existing.ID); err != nil {
		return fmt.Errorf("failed to delete Cas service for consumer %q: %w", reg.Consumer, err)
	}

	return nil
}

func findServiceByConsumer(services []RegisteredService, consumer string) (*RegisteredService, error) {
	var match *RegisteredService

	for i := range services {
		if services[i].Properties.GetFirstValue("ServiceName") != consumer {
			continue
		}

		if match != nil {
			return nil, fmt.Errorf("multiple Cas services found for consumer %q", consumer)
		}

		match = &services[i]
	}

	return match, nil
}

func (b *CASServiceRegistrationBackend) resolveClientSecret(reg domain.Registration, existingData domain.RegistrationData) (string, error) {
	if reg.Protocol == domain.ProtocolCAS {
		return "", nil
	}

	existingClientSecret, ok := existingData.ClientSecret(reg.Protocol)
	if !ok {
		return "", fmt.Errorf("unsupported protocol %q", reg.Protocol)
	}
	if existingClientSecret != "" {
		return existingClientSecret, nil
	}

	return generateClientSecret()
}

func generateClientSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := randomRead(bytes); err != nil {
		return "", fmt.Errorf("failed to generate client secret: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
