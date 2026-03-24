package cas

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path"
	"strings"

	"github.com/cloudogu/k8s-auth-registration-operator/internal/domain"
)

const fqdnKey = "fqdn"

type serviceMapper struct {
	globalConfigRepo globalConfigRepo
	casBasePath      string
}

func newServiceMapper(globalConfigRepo globalConfigRepo, casBasePath string) *serviceMapper {
	return &serviceMapper{
		globalConfigRepo: globalConfigRepo,
		casBasePath:      casBasePath,
	}
}

func (m *serviceMapper) ValidateRegistration(reg domain.Registration) error {
	if strings.TrimSpace(reg.Consumer) == "" {
		return fmt.Errorf("consumer must not be empty")
	}

	switch reg.Protocol {
	case domain.ProtocolCAS, domain.ProtocolOIDC, domain.ProtocolOAuth:
	default:
		return fmt.Errorf("unsupported protocol %q", reg.Protocol)
	}

	return nil
}

func (m *serviceMapper) BuildServicePayload(ctx context.Context, reg domain.Registration, existing *RegisteredService, services []RegisteredService, clientSecret string) (RegisteredService, error) {
	fqdn, err := m.getFQDN(ctx)
	if err != nil {
		return RegisteredService{}, fmt.Errorf("failed to get fqdn: %w", err)
	}

	service := RegisteredService{
		ID:           serviceId(existing),
		Class:        serviceClass(reg.Protocol),
		TemplateName: templateName(reg.Protocol, reg.LogoutURL != ""),
		Name:         reg.Consumer,
		ServiceID:    registrationServiceID(fqdn, reg.Consumer),
	}

	properties := NewRegisteredServiceProperties()
	properties.Entries["ServiceClass"] = NewRegisteredServiceProperty(service.Class)
	properties.Entries["Fqdn"] = NewRegisteredServiceProperty(escapeDots(fqdn))
	properties.Entries["ServiceName"] = NewRegisteredServiceProperty(reg.Consumer)

	logoutURL := logoutURLForRegistration(reg, fqdn)
	properties.Entries["LogoutUrl"] = NewRegisteredServiceProperty(logoutURL)

	service.Properties = properties

	switch reg.Protocol {
	case domain.ProtocolCAS:
	case domain.ProtocolOIDC:
		addOIDCFields(&service, reg, existing, clientSecret)
	case domain.ProtocolOAuth:
		addOAuthFields(&service, reg, existing, clientSecret)
	}

	return service, nil
}

func (m *serviceMapper) BuildRegistrationResult(reg domain.Registration, service RegisteredService, clientSecret string) (domain.RegistrationResult, error) {
	fqdn, err := m.getFQDN(context.Background())
	if err != nil {
		return domain.RegistrationResult{}, fmt.Errorf("failed to get fqdn: %w", err)
	}
	baseURL := fmt.Sprintf("https://%s/%s", fqdn, m.casBasePath)

	result := domain.RegistrationResult{
		Protocol:       reg.Protocol,
		RegistrationID: fmt.Sprintf("%d", service.ID),
	}

	switch reg.Protocol {
	case domain.ProtocolCAS:
		result.CAS = &domain.CASResult{
			ServiceID: service.ServiceID,
		}
	case domain.ProtocolOIDC:
		result.OIDC = &domain.OIDCResult{
			ClientID:     service.ClientID,
			ClientSecret: clientSecret,
			IssuerURL:    strings.TrimRight(m.casBasePath, "/") + "/oidc",
		}
	case domain.ProtocolOAuth:
		baseURL := strings.TrimRight(baseURL, "/")
		result.OAuth = &domain.OAuthResult{
			ClientID:     service.ClientID,
			ClientSecret: clientSecret,
			AuthURL:      baseURL + "/oauth2.0/authorize",
			TokenURL:     baseURL + "/oauth2.0/accessToken",
		}
	}

	return result, nil
}

func templateName(protocol domain.Protocol, hasLogoutURL bool) string {
	baseTemplates := []string{"BaseService", "DefaultAttributeReleasePolicy"}

	switch protocol {
	case domain.ProtocolCAS:
		return strings.Join(append(baseTemplates, "AllowProxyPolicy"), ",")
	case domain.ProtocolOIDC, domain.ProtocolOAuth:
		if hasLogoutURL {
			baseTemplates = append(baseTemplates, "WithLogoutURI")
		}
		baseTemplates = append(baseTemplates, "DefaultOAuthService")
		return strings.Join(baseTemplates, ",")
	default:
		return strings.Join(baseTemplates, ",")
	}
}

func serviceClass(protocol domain.Protocol) string {
	switch protocol {
	case domain.ProtocolCAS:
		return casRegisteredServiceClass
	case domain.ProtocolOIDC:
		return oidcRegisteredServiceClass
	case domain.ProtocolOAuth:
		return oauthRegisteredServiceClass
	default:
		return ""
	}
}

func addOIDCFields(service *RegisteredService, reg domain.Registration, existing *RegisteredService, clientSecret string) {
	service.ClientID = resolveClientID(reg, existing)
	service.ClientSecret = hashClientSecret(clientSecret)
	service.Scopes = newSetParam(reg.Params, "scopes")
	service.SupportedGrantTypes = newSetParam(reg.Params, "supportedGrantTypes")
	service.SupportedResponseTypes = newSetParam(reg.Params, "supportedResponseTypes")
}

func addOAuthFields(service *RegisteredService, reg domain.Registration, existing *RegisteredService, clientSecret string) {
	addOIDCFields(service, reg, existing, clientSecret)
	service.Audience = newSetParam(reg.Params, "audience")
}

func newSetParam(params map[string]string, sourceKey string) *StringCollection {
	raw := strings.TrimSpace(params[sourceKey])
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			values = append(values, trimmed)
		}
	}
	if len(values) == 0 {
		return nil
	}

	collection := NewStringCollection(hashSetClass, values...)
	return &collection
}

func logoutURLForRegistration(reg domain.Registration, fqdn string) string {
	logoutPath := path.Join(strings.TrimSpace(reg.Consumer), strings.TrimSpace(reg.LogoutURL))
	return fmt.Sprintf("https://%s/%s", fqdn, logoutPath)
}

func registrationServiceID(fqdn string, serviceName string) string {
	return fmt.Sprintf("^https://%s/%s(/.*)?$", escapeDots(fqdn), serviceName)
}

func resolveClientID(reg domain.Registration, existing *RegisteredService) string {
	if clientID := strings.TrimSpace(reg.Params["clientId"]); clientID != "" {
		return clientID
	}

	if existing != nil && strings.TrimSpace(existing.ClientID) != "" {
		return existing.ClientID
	}

	return reg.Consumer
}

func (m *serviceMapper) getFQDN(ctx context.Context) (string, error) {
	globalConfig, err := m.globalConfigRepo.Get(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get global config: %w", err)
	}

	fqdn, ok := globalConfig.Get(fqdnKey)
	if !ok {
		return "", fmt.Errorf("global config does not contain key %q", fqdnKey)
	}

	return fqdn.String(), nil
}

func serviceId(existing *RegisteredService) int64 {
	if existing == nil {
		return 0
	}

	return existing.ID
}

func hashClientSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return fmt.Sprintf("%x", sum[:])
}

func escapeDots(value string) string {
	return strings.ReplaceAll(value, ".", "[.]")
}
