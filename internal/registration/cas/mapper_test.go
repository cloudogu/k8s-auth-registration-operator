package cas

import (
	"context"
	"crypto/sha256"
	"fmt"
	"regexp"
	"testing"

	"github.com/cloudogu/k8s-auth-registration-operator/internal/domain"
	libconfig "github.com/cloudogu/k8s-registry-lib/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestLogoutURLForRegistration(t *testing.T) {
	t.Run("trims duplicate slashes between consumer and logout path", func(t *testing.T) {
		reg := domain.Registration{
			Consumer:  "bluespice",
			LogoutURL: "/w/rest.php/pluggableauth/logout",
		}

		actual := logoutURLForRegistration(reg, "dev1.k3ces.localdomain")

		assert.Equal(t, "https://dev1.k3ces.localdomain/bluespice/w/rest.php/pluggableauth/logout", actual)
	})
}

func TestServiceMapperValidateRegistration(t *testing.T) {
	mapper := serviceMapper{}

	assert.NoError(t, mapper.ValidateRegistration(domain.Registration{Protocol: domain.ProtocolCAS, Consumer: "cas-app"}))
	assert.NoError(t, mapper.ValidateRegistration(domain.Registration{Protocol: domain.ProtocolOIDC, Consumer: "oidc-app"}))
	assert.NoError(t, mapper.ValidateRegistration(domain.Registration{Protocol: domain.ProtocolOAuth, Consumer: "oauth-app"}))
	assert.ErrorContains(t, mapper.ValidateRegistration(domain.Registration{Protocol: domain.ProtocolCAS, Consumer: "  "}), "consumer must not be empty")
	assert.ErrorContains(t, mapper.ValidateRegistration(domain.Registration{Protocol: domain.Protocol("SAML"), Consumer: "app"}), `unsupported protocol "SAML"`)
}

func TestServiceMapperBuildServicePayload(t *testing.T) {
	t.Run("builds CAS payload", func(t *testing.T) {
		repo := newMockGlobalConfigRepo(t)
		repo.EXPECT().
			Get(mock.Anything).
			Return(libconfig.CreateGlobalConfig(libconfig.Entries{libconfig.Key("fqdn"): libconfig.Value("dev1.k3ces.localdomain")}), nil).
			Once()
		mapper := newServiceMapper(repo, "/cas")
		reg := domain.Registration{
			Protocol:  domain.ProtocolCAS,
			Consumer:  "bluespice",
			LogoutURL: "/logout",
		}

		service, err := mapper.BuildServicePayload(context.Background(), reg, nil, nil, "")

		require.NoError(t, err)
		assert.Equal(t, casRegisteredServiceClass, service.Class)
		assert.Equal(t, "BaseService,DefaultAttributeReleasePolicy,AllowProxyPolicy", service.TemplateName)
		assert.Equal(t, "bluespice", service.Name)
		assert.Equal(t, "^https://dev1[.]k3ces[.]localdomain(:[0-9]+)?/bluespice(/.*)?$", service.ServiceID)
		require.NotNil(t, service.Properties)
		assert.Equal(t, "bluespice", service.Properties.GetFirstValue("ServiceName"))
		assert.Equal(t, "dev1[.]k3ces[.]localdomain", service.Properties.GetFirstValue("Fqdn"))
		assert.Equal(t, "https://dev1.k3ces.localdomain/bluespice/logout", service.Properties.GetFirstValue("LogoutUrl"))
	})

	t.Run("builds OIDC payload using existing client id and current secret", func(t *testing.T) {
		repo := newMockGlobalConfigRepo(t)
		repo.EXPECT().
			Get(mock.Anything).
			Return(libconfig.CreateGlobalConfig(libconfig.Entries{libconfig.Key("fqdn"): libconfig.Value("dev1.k3ces.localdomain")}), nil).
			Once()
		mapper := newServiceMapper(repo, "/cas")
		reg := domain.Registration{
			Protocol:  domain.ProtocolOIDC,
			Consumer:  "app",
			LogoutURL: "logout",
			Params: map[string]string{
				"scopes":                 "openid, profile",
				"supportedGrantTypes":    "authorization_code",
				"supportedResponseTypes": "code",
			},
		}
		existing := &RegisteredService{ID: 77, ClientID: "existing-client-id"}

		service, err := mapper.BuildServicePayload(context.Background(), reg, existing, nil, "plain-secret")

		require.NoError(t, err)
		assert.Equal(t, int64(77), service.ID)
		assert.Equal(t, oidcRegisteredServiceClass, service.Class)
		assert.Equal(t, "BaseService,DefaultAttributeReleasePolicy,WithLogoutURI,DefaultOAuthService", service.TemplateName)
		assert.Equal(t, "existing-client-id", service.ClientID)
		assert.Equal(t, fmt.Sprintf("%x", sha256.Sum256([]byte("plain-secret"))), service.ClientSecret)
		require.NotNil(t, service.Scopes)
		assert.Equal(t, []string{"openid", "profile"}, service.Scopes.Values)
		require.NotNil(t, service.SupportedGrantTypes)
		assert.Equal(t, []string{"authorization_code"}, service.SupportedGrantTypes.Values)
		require.NotNil(t, service.SupportedResponseTypes)
		assert.Equal(t, []string{"code"}, service.SupportedResponseTypes.Values)
	})

	t.Run("builds OAuth payload using explicit client id", func(t *testing.T) {
		repo := newMockGlobalConfigRepo(t)
		repo.EXPECT().
			Get(mock.Anything).
			Return(libconfig.CreateGlobalConfig(libconfig.Entries{libconfig.Key("fqdn"): libconfig.Value("dev1.k3ces.localdomain")}), nil).
			Once()
		mapper := newServiceMapper(repo, "/cas")
		reg := domain.Registration{
			Protocol: domain.ProtocolOAuth,
			Consumer: "app",
			Params: map[string]string{
				"clientId":               "explicit-client-id",
				"scopes":                 "openid",
				"supportedGrantTypes":    "client_credentials",
				"supportedResponseTypes": "token",
				"audience":               "api, profile",
			},
		}

		service, err := mapper.BuildServicePayload(context.Background(), reg, nil, nil, "plain-secret")

		require.NoError(t, err)
		assert.Equal(t, oauthRegisteredServiceClass, service.Class)
		assert.Equal(t, "BaseService,DefaultAttributeReleasePolicy,DefaultOAuthService", service.TemplateName)
		assert.Equal(t, "explicit-client-id", service.ClientID)
		require.NotNil(t, service.Audience)
		assert.Equal(t, []string{"api", "profile"}, service.Audience.Values)
	})

	t.Run("returns fqdn errors from the repository", func(t *testing.T) {
		repo := newMockGlobalConfigRepo(t)
		repo.EXPECT().Get(mock.Anything).Return(libconfig.GlobalConfig{}, assert.AnError).Once()
		mapper := newServiceMapper(repo, "/cas")

		service, err := mapper.BuildServicePayload(context.Background(), domain.Registration{Protocol: domain.ProtocolCAS, Consumer: "app"}, nil, nil, "")

		require.Error(t, err)
		assert.Empty(t, service)
		assert.ErrorContains(t, err, "failed to get fqdn")
	})
}

func TestServiceMapperBuildRegistrationResult(t *testing.T) {
	t.Run("builds CAS OIDC and OAuth results", func(t *testing.T) {
		repo := newMockGlobalConfigRepo(t)
		repo.EXPECT().
			Get(mock.Anything).
			Return(libconfig.CreateGlobalConfig(libconfig.Entries{libconfig.Key("fqdn"): libconfig.Value("dev1.k3ces.localdomain")}), nil).
			Times(3)
		mapper := newServiceMapper(repo, "/cas")

		casResult, err := mapper.BuildRegistrationResult(
			domain.Registration{Protocol: domain.ProtocolCAS},
			RegisteredService{ID: 11, ServiceID: "^https://cas.example.com/app(/.*)?$"},
			"",
		)
		require.NoError(t, err)
		assert.Equal(t, "11", casResult.RegistrationID)
		require.NotNil(t, casResult.CAS)
		assert.Equal(t, "^https://cas.example.com/app(/.*)?$", casResult.CAS.ServiceID)

		oidcResult, err := mapper.BuildRegistrationResult(
			domain.Registration{Protocol: domain.ProtocolOIDC},
			RegisteredService{ID: 12, ClientID: "oidc-client"},
			"plain-secret",
		)
		require.NoError(t, err)
		require.NotNil(t, oidcResult.OIDC)
		assert.Equal(t, "oidc-client", oidcResult.OIDC.ClientID)
		assert.Equal(t, "plain-secret", oidcResult.OIDC.ClientSecret)
		assert.Equal(t, "/cas/oidc", oidcResult.OIDC.IssuerURL)

		oauthResult, err := mapper.BuildRegistrationResult(
			domain.Registration{Protocol: domain.ProtocolOAuth},
			RegisteredService{ID: 13, ClientID: "oauth-client"},
			"plain-secret",
		)
		require.NoError(t, err)
		require.NotNil(t, oauthResult.OAuth)
		assert.Equal(t, "oauth-client", oauthResult.OAuth.ClientID)
		assert.Equal(t, "plain-secret", oauthResult.OAuth.ClientSecret)
		assert.Equal(t, "https://dev1.k3ces.localdomain/cas/oauth2.0/authorize", oauthResult.OAuth.AuthURL)
		assert.Equal(t, "https://dev1.k3ces.localdomain/cas/oauth2.0/accessToken", oauthResult.OAuth.TokenURL)
	})

	t.Run("returns fqdn errors from the repository", func(t *testing.T) {
		repo := newMockGlobalConfigRepo(t)
		repo.EXPECT().Get(mock.Anything).Return(libconfig.GlobalConfig{}, assert.AnError).Once()
		mapper := newServiceMapper(repo, "/cas")

		result, err := mapper.BuildRegistrationResult(domain.Registration{Protocol: domain.ProtocolCAS}, RegisteredService{}, "")

		require.Error(t, err)
		assert.Empty(t, result)
		assert.ErrorContains(t, err, "failed to get fqdn")
	})
}

func TestServiceMapperHelperFunctions(t *testing.T) {
	t.Run("cover protocol and path handling helpers", func(t *testing.T) {
		assert.Equal(t, "BaseService,DefaultAttributeReleasePolicy,AllowProxyPolicy", templateName(domain.ProtocolCAS, false))
		assert.Equal(t, "BaseService,DefaultAttributeReleasePolicy,WithLogoutURI,DefaultOAuthService", templateName(domain.ProtocolOIDC, true))
		assert.Equal(t, "BaseService,DefaultAttributeReleasePolicy,DefaultOAuthService", templateName(domain.ProtocolOAuth, false))
		assert.Equal(t, "BaseService,DefaultAttributeReleasePolicy", templateName(domain.Protocol("UNKNOWN"), false))
		assert.Equal(t, "", serviceClass(domain.Protocol("UNKNOWN")))
		assert.Nil(t, newSetParam(map[string]string{"scopes": "   "}, "scopes"))
		assert.Nil(t, newSetParam(map[string]string{"scopes": " , , "}, "scopes"))
		require.NotNil(t, newSetParam(map[string]string{"scopes": "openid, profile"}, "scopes"))
		pattern := registrationServiceID("dev1.k3ces.localdomain", "app")
		assert.Equal(t, "^https://dev1[.]k3ces[.]localdomain(:[0-9]+)?/app(/.*)?$", pattern)
		assert.True(t, regexp.MustCompile(pattern).MatchString("https://dev1.k3ces.localdomain/app"))
		assert.True(t, regexp.MustCompile(pattern).MatchString("https://dev1.k3ces.localdomain:443/app/api/v2/cas/auth"))
		assert.True(t, regexp.MustCompile(pattern).MatchString("https://dev1.k3ces.localdomain:8443/app/api/v2/cas/auth"))
		assert.Equal(t, "explicit-client-id", resolveClientID(domain.Registration{Consumer: "app", Params: map[string]string{"clientId": "explicit-client-id"}}, nil))
		assert.Equal(t, "existing-client-id", resolveClientID(domain.Registration{Consumer: "app"}, &RegisteredService{ClientID: "existing-client-id"}))
		assert.Equal(t, "app", resolveClientID(domain.Registration{Consumer: "app"}, nil))
		assert.Equal(t, int64(0), serviceId(nil))
		assert.Equal(t, int64(55), serviceId(&RegisteredService{ID: 55}))
		assert.Equal(t, fmt.Sprintf("%x", sha256.Sum256([]byte("plain-secret"))), hashClientSecret("plain-secret"))
		assert.Equal(t, "dev1[.]k3ces[.]localdomain", escapeDots("dev1.k3ces.localdomain"))
		oidcSecret, oidcOK := domain.RegistrationData{"oidc_client_secret": []byte("oidc-secret")}.ClientSecret(domain.ProtocolOIDC)
		assert.True(t, oidcOK)
		assert.Equal(t, "oidc-secret", oidcSecret)
		oauthSecret, oauthOK := domain.RegistrationData{"oauth_client_secret": []byte("oauth-secret")}.ClientSecret(domain.ProtocolOAuth)
		assert.True(t, oauthOK)
		assert.Equal(t, "oauth-secret", oauthSecret)
		casSecret, casOK := domain.RegistrationData{"oidc_client_secret": []byte("oidc-secret")}.ClientSecret(domain.ProtocolCAS)
		assert.False(t, casOK)
		assert.Equal(t, "", casSecret)
	})

	t.Run("getFQDN returns missing key errors", func(t *testing.T) {
		repo := newMockGlobalConfigRepo(t)
		repo.EXPECT().Get(mock.Anything).Return(libconfig.CreateGlobalConfig(libconfig.Entries{}), nil).Once()
		mapper := newServiceMapper(repo, "/cas")

		fqdn, err := mapper.getFQDN(context.Background())

		require.Error(t, err)
		assert.Empty(t, fqdn)
		assert.ErrorContains(t, err, `global config does not contain key "fqdn"`)
	})
}
