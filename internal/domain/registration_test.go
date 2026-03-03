package domain

import (
	"testing"

	authregistrationv1 "github.com/cloudogu/k8s-auth-registration-lib/api/v1"
	"github.com/stretchr/testify/assert"
)

func TestRegistrationResult_GetSecretData(t *testing.T) {
	t.Run("should return CAS secret data", func(t *testing.T) {
		result := RegistrationResult{
			Protocol: ProtocolCAS,
			CAS: &CASResult{
				ServiceID: "service-id-1",
			},
		}

		secretData := result.GetSecretData()

		assert.Equal(t, map[string][]byte{
			"serviceId": []byte("service-id-1"),
		}, secretData)
	})

	t.Run("should return OIDC secret data", func(t *testing.T) {
		result := RegistrationResult{
			Protocol: ProtocolOIDC,
			OIDC: &OIDCResult{
				ClientID:     "client-id",
				ClientSecret: "client-secret",
				IssuerURL:    "https://issuer.example.com",
			},
		}

		secretData := result.GetSecretData()

		assert.Equal(t, map[string][]byte{
			"clientId":     []byte("client-id"),
			"clientSecret": []byte("client-secret"),
			"issuerUrl":    []byte("https://issuer.example.com"),
		}, secretData)
	})

	t.Run("should return OAuth secret data", func(t *testing.T) {
		result := RegistrationResult{
			Protocol: ProtocolOAuth,
			OAuth: &OAuthResult{
				ClientID:     "oauth-client-id",
				ClientSecret: "oauth-client-secret",
				AuthURL:      "https://auth.example.com",
				TokenURL:     "https://token.example.com",
			},
		}

		secretData := result.GetSecretData()

		assert.Equal(t, map[string][]byte{
			"clientId":     []byte("oauth-client-id"),
			"clientSecret": []byte("oauth-client-secret"),
			"authURL":      []byte("https://auth.example.com"),
			"tokenURL":     []byte("https://token.example.com"),
		}, secretData)
	})

	t.Run("should return empty map for unknown protocol", func(t *testing.T) {
		result := RegistrationResult{
			Protocol: Protocol("UNKNOWN"),
		}

		secretData := result.GetSecretData()

		assert.Empty(t, secretData)
	})

	t.Run("should return empty map for CAS protocol without CAS payload", func(t *testing.T) {
		result := RegistrationResult{
			Protocol: ProtocolCAS,
		}

		secretData := result.GetSecretData()

		assert.Empty(t, secretData)
	})

	t.Run("should return empty map for OIDC protocol without OIDC payload", func(t *testing.T) {
		result := RegistrationResult{
			Protocol: ProtocolOIDC,
		}

		secretData := result.GetSecretData()

		assert.Empty(t, secretData)
	})

	t.Run("should return empty map for OAuth protocol without OAuth payload", func(t *testing.T) {
		result := RegistrationResult{
			Protocol: ProtocolOAuth,
		}

		secretData := result.GetSecretData()

		assert.Empty(t, secretData)
	})
}

func TestFromAuthRegistration(t *testing.T) {
	t.Run("should map auth registration with logout URL", func(t *testing.T) {
		logoutURL := "https://logout.example.com"
		params := map[string]string{"scope": "openid profile"}

		authRegistration := &authregistrationv1.AuthRegistration{
			Spec: authregistrationv1.AuthRegistrationSpec{
				Protocol:  authregistrationv1.AuthProtocolOIDC,
				Consumer:  "my-consumer",
				LogoutURL: &logoutURL,
				Params:    params,
			},
		}

		registration := FromAuthRegistration(authRegistration)

		assert.Equal(t, Registration{
			Protocol:  ProtocolOIDC,
			Consumer:  "my-consumer",
			LogoutURL: "https://logout.example.com",
			Params:    params,
		}, registration)
	})

	t.Run("should map auth registration without logout URL to empty string", func(t *testing.T) {
		authRegistration := &authregistrationv1.AuthRegistration{
			Spec: authregistrationv1.AuthRegistrationSpec{
				Protocol: authregistrationv1.AuthProtocolCAS,
				Consumer: "cas-consumer",
				Params:   map[string]string{"service": "foo"},
			},
		}

		registration := FromAuthRegistration(authRegistration)

		assert.Equal(t, Registration{
			Protocol:  ProtocolCAS,
			Consumer:  "cas-consumer",
			LogoutURL: "",
			Params:    map[string]string{"service": "foo"},
		}, registration)
	})
}
