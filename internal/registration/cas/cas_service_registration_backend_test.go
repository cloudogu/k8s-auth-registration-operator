package cas

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/cloudogu/k8s-auth-registration-operator/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewCASServiceRegistrationBackend(t *testing.T) {
	t.Run("returns error when client is nil", func(t *testing.T) {
		backend, err := NewCASServiceRegistrationBackend(nil, nil)

		require.Error(t, err)
		assert.Nil(t, backend)
		assert.ErrorContains(t, err, "client must not be nil")
	})

	t.Run("returns error when client base URL is invalid", func(t *testing.T) {
		client := newMockApiClient(t)
		client.EXPECT().BaseURL().Return("http://[::1").Twice()

		backend, err := NewCASServiceRegistrationBackend(nil, client)

		require.Error(t, err)
		assert.Nil(t, backend)
		assert.ErrorContains(t, err, "invalid base URL")
	})

	t.Run("creates backend when client base URL is valid", func(t *testing.T) {
		client := newMockApiClient(t)
		client.EXPECT().BaseURL().Return("https://cas.example.com/cas").Once()

		backend, err := NewCASServiceRegistrationBackend(nil, client)

		require.NoError(t, err)
		require.NotNil(t, backend)
		assert.Same(t, client, backend.client)
		assert.NotNil(t, backend.mapper)
	})
}

func TestCASServiceRegistrationBackend_Upsert(t *testing.T) {
	ctx := context.Background()

	t.Run("returns validation error before calling the client", func(t *testing.T) {
		mapper := newMockRegistrationMapper(t)
		client := newMockApiClient(t)
		backend := &CASServiceRegistrationBackend{client: client, mapper: mapper}
		reg := domain.Registration{Protocol: domain.ProtocolOIDC, Consumer: "app"}
		expectedErr := errors.New("invalid registration")

		mapper.EXPECT().ValidateRegistration(reg).Return(expectedErr).Once()

		result, err := backend.Upsert(ctx, reg, nil)

		require.Error(t, err)
		assert.Empty(t, result)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("returns wrapped error when listing services fails", func(t *testing.T) {
		mapper := newMockRegistrationMapper(t)
		client := newMockApiClient(t)
		backend := &CASServiceRegistrationBackend{client: client, mapper: mapper}
		reg := domain.Registration{Protocol: domain.ProtocolOIDC, Consumer: "app"}
		expectedErr := errors.New("cas unavailable")

		mapper.EXPECT().ValidateRegistration(reg).Return(nil).Once()
		client.EXPECT().ListServices(ctx).Return(nil, expectedErr).Once()

		result, err := backend.Upsert(ctx, reg, nil)

		require.Error(t, err)
		assert.Empty(t, result)
		assert.ErrorContains(t, err, "failed to list Cas services")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("creates a service when no existing registration matches the consumer", func(t *testing.T) {
		mapper := newMockRegistrationMapper(t)
		client := newMockApiClient(t)
		backend := &CASServiceRegistrationBackend{client: client, mapper: mapper}
		reg := domain.Registration{Protocol: domain.ProtocolOIDC, Consumer: "app"}
		existingData := domain.RegistrationData{
			"oidc_client_secret": []byte("existing-client-secret"),
		}
		services := []RegisteredService{
			newRegisteredServiceForBackendTest(7, "different-app"),
		}
		payload := RegisteredService{Name: "app", ID: 101}
		persisted := RegisteredService{Name: "app", ID: 202}
		expectedResult := domain.RegistrationResult{
			Protocol:       domain.ProtocolOIDC,
			RegistrationID: "202",
		}

		mapper.EXPECT().ValidateRegistration(reg).Return(nil).Once()
		client.EXPECT().ListServices(ctx).Return(services, nil).Once()
		mapper.EXPECT().
			BuildServicePayload(ctx, reg, (*RegisteredService)(nil), services, "existing-client-secret").
			Return(payload, nil).
			Once()
		client.EXPECT().CreateService(ctx, payload).Return(persisted, nil).Once()
		mapper.EXPECT().
			BuildRegistrationResult(reg, persisted, "existing-client-secret").
			Return(expectedResult, nil).
			Once()

		result, err := backend.Upsert(ctx, reg, existingData)

		require.NoError(t, err)
		assert.Equal(t, expectedResult, result)
	})

	t.Run("updates the matching service when a registration already exists", func(t *testing.T) {
		mapper := newMockRegistrationMapper(t)
		client := newMockApiClient(t)
		backend := &CASServiceRegistrationBackend{client: client, mapper: mapper}
		reg := domain.Registration{Protocol: domain.ProtocolOAuth, Consumer: "app"}
		existingData := domain.RegistrationData{
			"oauth_client_secret": []byte("existing-oauth-secret"),
		}
		services := []RegisteredService{
			newRegisteredServiceForBackendTest(42, "app"),
		}
		payload := RegisteredService{Name: "app", ID: 42}
		persisted := RegisteredService{Name: "app", ID: 42}
		expectedResult := domain.RegistrationResult{
			Protocol:       domain.ProtocolOAuth,
			RegistrationID: "42",
		}

		mapper.EXPECT().ValidateRegistration(reg).Return(nil).Once()
		client.EXPECT().ListServices(ctx).Return(services, nil).Once()
		mapper.EXPECT().
			BuildServicePayload(
				ctx,
				reg,
				mock.MatchedBy(func(existing *RegisteredService) bool {
					return existing != nil && existing.ID == 42
				}),
				services,
				"existing-oauth-secret",
			).
			Return(payload, nil).
			Once()
		client.EXPECT().UpdateService(ctx, payload).Return(persisted, nil).Once()
		mapper.EXPECT().
			BuildRegistrationResult(reg, persisted, "existing-oauth-secret").
			Return(expectedResult, nil).
			Once()

		result, err := backend.Upsert(ctx, reg, existingData)

		require.NoError(t, err)
		assert.Equal(t, expectedResult, result)
	})

	t.Run("returns error when multiple services match the same consumer", func(t *testing.T) {
		mapper := newMockRegistrationMapper(t)
		client := newMockApiClient(t)
		backend := &CASServiceRegistrationBackend{client: client, mapper: mapper}
		reg := domain.Registration{Protocol: domain.ProtocolOIDC, Consumer: "app"}
		services := []RegisteredService{
			newRegisteredServiceForBackendTest(1, "app"),
			newRegisteredServiceForBackendTest(2, "app"),
		}

		mapper.EXPECT().ValidateRegistration(reg).Return(nil).Once()
		client.EXPECT().ListServices(ctx).Return(services, nil).Once()

		result, err := backend.Upsert(ctx, reg, nil)

		require.Error(t, err)
		assert.Empty(t, result)
		assert.ErrorContains(t, err, `multiple Cas services found for consumer "app"`)
	})

	t.Run("returns wrapped error when creating a service fails", func(t *testing.T) {
		mapper := newMockRegistrationMapper(t)
		client := newMockApiClient(t)
		backend := &CASServiceRegistrationBackend{client: client, mapper: mapper}
		reg := domain.Registration{Protocol: domain.ProtocolOIDC, Consumer: "app"}
		existingData := domain.RegistrationData{
			"oidc_client_secret": []byte("existing-client-secret"),
		}
		payload := RegisteredService{Name: "app", ID: 101}
		expectedErr := errors.New("create failed")

		mapper.EXPECT().ValidateRegistration(reg).Return(nil).Once()
		client.EXPECT().ListServices(ctx).Return([]RegisteredService{}, nil).Once()
		mapper.EXPECT().
			BuildServicePayload(ctx, reg, (*RegisteredService)(nil), []RegisteredService{}, "existing-client-secret").
			Return(payload, nil).
			Once()
		client.EXPECT().CreateService(ctx, payload).Return(RegisteredService{}, expectedErr).Once()

		result, err := backend.Upsert(ctx, reg, existingData)

		require.Error(t, err)
		assert.Empty(t, result)
		assert.ErrorContains(t, err, `failed to create Cas service for consumer "app"`)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("returns error when building the service payload fails", func(t *testing.T) {
		mapper := newMockRegistrationMapper(t)
		client := newMockApiClient(t)
		backend := &CASServiceRegistrationBackend{client: client, mapper: mapper}
		reg := domain.Registration{Protocol: domain.ProtocolOIDC, Consumer: "app"}
		existingData := domain.RegistrationData{
			"oidc_client_secret": []byte("existing-client-secret"),
		}
		expectedErr := errors.New("payload failed")

		mapper.EXPECT().ValidateRegistration(reg).Return(nil).Once()
		client.EXPECT().ListServices(ctx).Return([]RegisteredService{}, nil).Once()
		mapper.EXPECT().
			BuildServicePayload(ctx, reg, (*RegisteredService)(nil), []RegisteredService{}, "existing-client-secret").
			Return(RegisteredService{}, expectedErr).
			Once()

		result, err := backend.Upsert(ctx, reg, existingData)

		require.Error(t, err)
		assert.Empty(t, result)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("returns wrapped error when updating a service fails", func(t *testing.T) {
		mapper := newMockRegistrationMapper(t)
		client := newMockApiClient(t)
		backend := &CASServiceRegistrationBackend{client: client, mapper: mapper}
		reg := domain.Registration{Protocol: domain.ProtocolOAuth, Consumer: "app"}
		existingData := domain.RegistrationData{
			"oauth_client_secret": []byte("existing-oauth-secret"),
		}
		services := []RegisteredService{
			newRegisteredServiceForBackendTest(42, "app"),
		}
		payload := RegisteredService{Name: "app", ID: 42}
		expectedErr := errors.New("update failed")

		mapper.EXPECT().ValidateRegistration(reg).Return(nil).Once()
		client.EXPECT().ListServices(ctx).Return(services, nil).Once()
		mapper.EXPECT().
			BuildServicePayload(
				ctx,
				reg,
				mock.MatchedBy(func(existing *RegisteredService) bool {
					return existing != nil && existing.ID == 42
				}),
				services,
				"existing-oauth-secret",
			).
			Return(payload, nil).
			Once()
		client.EXPECT().UpdateService(ctx, payload).Return(RegisteredService{}, expectedErr).Once()

		result, err := backend.Upsert(ctx, reg, existingData)

		require.Error(t, err)
		assert.Empty(t, result)
		assert.ErrorContains(t, err, `failed to update Cas service for consumer "app"`)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("returns error when building the registration result fails", func(t *testing.T) {
		mapper := newMockRegistrationMapper(t)
		client := newMockApiClient(t)
		backend := &CASServiceRegistrationBackend{client: client, mapper: mapper}
		reg := domain.Registration{Protocol: domain.ProtocolOIDC, Consumer: "app"}
		existingData := domain.RegistrationData{
			"oidc_client_secret": []byte("existing-client-secret"),
		}
		payload := RegisteredService{Name: "app", ID: 101}
		persisted := RegisteredService{Name: "app", ID: 202}
		expectedErr := errors.New("result failed")

		mapper.EXPECT().ValidateRegistration(reg).Return(nil).Once()
		client.EXPECT().ListServices(ctx).Return([]RegisteredService{}, nil).Once()
		mapper.EXPECT().
			BuildServicePayload(ctx, reg, (*RegisteredService)(nil), []RegisteredService{}, "existing-client-secret").
			Return(payload, nil).
			Once()
		client.EXPECT().CreateService(ctx, payload).Return(persisted, nil).Once()
		mapper.EXPECT().
			BuildRegistrationResult(reg, persisted, "existing-client-secret").
			Return(domain.RegistrationResult{}, expectedErr).
			Once()

		result, err := backend.Upsert(ctx, reg, existingData)

		require.Error(t, err)
		assert.Empty(t, result)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("generates a new client secret when no secret exists in registration data", func(t *testing.T) {
		mapper := newMockRegistrationMapper(t)
		client := newMockApiClient(t)
		backend := &CASServiceRegistrationBackend{client: client, mapper: mapper}
		reg := domain.Registration{Protocol: domain.ProtocolOIDC, Consumer: "app"}
		payload := RegisteredService{Name: "app", ID: 101}
		persisted := RegisteredService{Name: "app", ID: 202}
		expectedResult := domain.RegistrationResult{
			Protocol:       domain.ProtocolOIDC,
			RegistrationID: "202",
		}

		mapper.EXPECT().ValidateRegistration(reg).Return(nil).Once()
		client.EXPECT().ListServices(ctx).Return([]RegisteredService{}, nil).Once()
		mapper.EXPECT().
			BuildServicePayload(
				ctx,
				reg,
				(*RegisteredService)(nil),
				[]RegisteredService{},
				mock.MatchedBy(func(secret string) bool { return len(secret) == 43 }),
			).
			Return(payload, nil).
			Once()
		client.EXPECT().CreateService(ctx, payload).Return(persisted, nil).Once()
		mapper.EXPECT().
			BuildRegistrationResult(
				reg,
				persisted,
				mock.MatchedBy(func(secret string) bool { return len(secret) == 43 }),
			).
			Return(expectedResult, nil).
			Once()

		result, err := backend.Upsert(ctx, reg, domain.RegistrationData{})

		require.NoError(t, err)
		assert.Equal(t, expectedResult, result)
	})

	t.Run("returns error when generating a new client secret fails", func(t *testing.T) {
		originalRandomRead := randomRead
		randomRead = func([]byte) (int, error) {
			return 0, io.EOF
		}
		t.Cleanup(func() {
			randomRead = originalRandomRead
		})

		mapper := newMockRegistrationMapper(t)
		client := newMockApiClient(t)
		backend := &CASServiceRegistrationBackend{client: client, mapper: mapper}
		reg := domain.Registration{Protocol: domain.ProtocolOIDC, Consumer: "app"}

		mapper.EXPECT().ValidateRegistration(reg).Return(nil).Once()
		client.EXPECT().ListServices(ctx).Return([]RegisteredService{}, nil).Once()

		result, err := backend.Upsert(ctx, reg, domain.RegistrationData{})

		require.Error(t, err)
		assert.Empty(t, result)
		assert.ErrorContains(t, err, "failed to generate client secret")
		assert.ErrorIs(t, err, io.EOF)
	})
}

func TestCASServiceRegistrationBackend_Delete(t *testing.T) {
	ctx := context.Background()

	t.Run("returns error when consumer is empty", func(t *testing.T) {
		backend := &CASServiceRegistrationBackend{client: newMockApiClient(t)}

		err := backend.Delete(ctx, domain.Registration{Consumer: "   "})

		require.Error(t, err)
		assert.ErrorContains(t, err, "consumer must not be empty")
	})

	t.Run("returns wrapped error when listing services fails", func(t *testing.T) {
		client := newMockApiClient(t)
		backend := &CASServiceRegistrationBackend{client: client}
		reg := domain.Registration{Consumer: "app"}
		expectedErr := errors.New("list failed")

		client.EXPECT().ListServices(ctx).Return(nil, expectedErr).Once()

		err := backend.Delete(ctx, reg)

		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to list Cas services")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("returns nil when no matching service exists", func(t *testing.T) {
		client := newMockApiClient(t)
		backend := &CASServiceRegistrationBackend{client: client}
		reg := domain.Registration{Consumer: "app"}
		services := []RegisteredService{
			newRegisteredServiceForBackendTest(7, "different-app"),
		}

		client.EXPECT().ListServices(ctx).Return(services, nil).Once()

		err := backend.Delete(ctx, reg)

		require.NoError(t, err)
	})

	t.Run("returns error when multiple services match the same consumer", func(t *testing.T) {
		client := newMockApiClient(t)
		backend := &CASServiceRegistrationBackend{client: client}
		reg := domain.Registration{Consumer: "app"}
		services := []RegisteredService{
			newRegisteredServiceForBackendTest(1, "app"),
			newRegisteredServiceForBackendTest(2, "app"),
		}

		client.EXPECT().ListServices(ctx).Return(services, nil).Once()

		err := backend.Delete(ctx, reg)

		require.Error(t, err)
		assert.ErrorContains(t, err, `multiple Cas services found for consumer "app"`)
	})

	t.Run("returns error when the matching service has no numeric id", func(t *testing.T) {
		client := newMockApiClient(t)
		backend := &CASServiceRegistrationBackend{client: client}
		reg := domain.Registration{Consumer: "app"}
		services := []RegisteredService{
			newRegisteredServiceForBackendTest(0, "app"),
		}

		client.EXPECT().ListServices(ctx).Return(services, nil).Once()

		err := backend.Delete(ctx, reg)

		require.Error(t, err)
		assert.ErrorContains(t, err, `cas-service for consumer "app" has no numeric id`)
	})

	t.Run("deletes the matching service by numeric id", func(t *testing.T) {
		client := newMockApiClient(t)
		backend := &CASServiceRegistrationBackend{client: client}
		reg := domain.Registration{Consumer: "app"}
		services := []RegisteredService{
			newRegisteredServiceForBackendTest(42, "app"),
		}

		client.EXPECT().ListServices(ctx).Return(services, nil).Once()
		client.EXPECT().DeleteService(ctx, int64(42)).Return(nil).Once()

		err := backend.Delete(ctx, reg)

		require.NoError(t, err)
	})

	t.Run("returns wrapped error when deleting the matching service fails", func(t *testing.T) {
		client := newMockApiClient(t)
		backend := &CASServiceRegistrationBackend{client: client}
		reg := domain.Registration{Consumer: "app"}
		services := []RegisteredService{
			newRegisteredServiceForBackendTest(42, "app"),
		}
		expectedErr := errors.New("delete failed")

		client.EXPECT().ListServices(ctx).Return(services, nil).Once()
		client.EXPECT().DeleteService(ctx, int64(42)).Return(expectedErr).Once()

		err := backend.Delete(ctx, reg)

		require.Error(t, err)
		assert.ErrorContains(t, err, `failed to delete Cas service for consumer "app"`)
		assert.ErrorIs(t, err, expectedErr)
	})
}

func TestCASServiceRegistrationBackendResolveClientSecret(t *testing.T) {
	t.Run("returns empty secret for CAS protocol", func(t *testing.T) {
		backend := &CASServiceRegistrationBackend{}

		secret, err := backend.resolveClientSecret(domain.Registration{Protocol: domain.ProtocolCAS}, nil)

		require.NoError(t, err)
		assert.Empty(t, secret)
	})

	t.Run("returns existing OIDC client secret from registration data", func(t *testing.T) {
		backend := &CASServiceRegistrationBackend{}

		secret, err := backend.resolveClientSecret(
			domain.Registration{Protocol: domain.ProtocolOIDC},
			domain.RegistrationData{"oidc_client_secret": []byte("existing-secret")},
		)

		require.NoError(t, err)
		assert.Equal(t, "existing-secret", secret)
	})

	t.Run("generates a new secret when no OAuth client secret exists", func(t *testing.T) {
		backend := &CASServiceRegistrationBackend{}

		secret, err := backend.resolveClientSecret(
			domain.Registration{Protocol: domain.ProtocolOAuth},
			domain.RegistrationData{},
		)

		require.NoError(t, err)
		assert.Len(t, secret, 43)
	})
}

func TestGenerateClientSecret(t *testing.T) {
	t.Run("creates base64url encoded secret", func(t *testing.T) {
		secret, err := generateClientSecret()

		require.NoError(t, err)
		assert.Len(t, secret, 43)
		assert.NotContains(t, secret, "=")
	})

	t.Run("returns wrapped error when random source fails", func(t *testing.T) {
		originalRandomRead := randomRead
		randomRead = func([]byte) (int, error) {
			return 0, io.EOF
		}
		t.Cleanup(func() {
			randomRead = originalRandomRead
		})

		secret, err := generateClientSecret()

		require.Error(t, err)
		assert.Empty(t, secret)
		assert.ErrorContains(t, err, "failed to generate client secret")
		assert.ErrorIs(t, err, io.EOF)
	})
}

func newRegisteredServiceForBackendTest(id int64, consumer string) RegisteredService {
	properties := NewRegisteredServiceProperties()
	properties.Entries["ServiceName"] = NewRegisteredServiceProperty(consumer)

	return RegisteredService{
		ID:         id,
		Name:       consumer,
		Properties: properties,
	}
}
