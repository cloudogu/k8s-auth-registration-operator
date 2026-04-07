package cas

import (
	"bytes"
	"encoding/json"
	"fmt"
)

const (
	javaCollectionWrapperLength = 2
	classJSONKey                = "@class"
	casRegisteredServiceClass   = "org.apereo.cas.services.CasRegisteredService"
	oidcRegisteredServiceClass  = "org.apereo.cas.services.OidcRegisteredService"
	oauthRegisteredServiceClass = "org.apereo.cas.support.oauth.services.OAuthRegisteredService"
	hashMapClass                = "java.util.HashMap"
	hashSetClass                = "java.util.HashSet"
	linkedHashSetClass          = "java.util.LinkedHashSet"
	servicePropertyClass        = "org.apereo.cas.services.DefaultRegisteredServiceProperty"
)

// RegisteredService represents an enable authentication web service as registered in CAS, independent of the used
// authentication service provide mechanism, f. i. OIDC.
//
// see also
//   - https://apereo.github.io/cas/7.3.x/authentication/OIDC-Authentication-Clients.html
//   - https://apereo.github.io/cas/7.3.x/authentication/OAuth-Authentication-Clients.html
type RegisteredService struct {
	// Class contains the CAS-specific Java class representing this authentication entity.
	Class        string `json:"@class"`
	ID           int64  `json:"id,omitempty"`
	TemplateName string `json:"templateName,omitempty"`
	// Name is required for services managed via the CAS registeredServices API.
	Name string `json:"name,omitempty"`
	// ServiceID is required for services managed via the CAS registeredServices API.
	ServiceID string `json:"serviceId,omitempty"`
	// LogoutURL contains the URL under to which CAS sends back-channel logout requests.
	LogoutURL string `json:"logoutUrl,omitempty"`
	// Properties may contain arbitrary control properties specific to the used authentication service provide mechanism.
	Properties *RegisteredServiceProperties `json:"properties,omitempty"`
	// ClientID contains the identifier for this client application.
	ClientID string `json:"clientId,omitempty"`
	// ClientSecret contains the secret for this client application. The client secret received from the service will be
	// URL decoded before being compared to the secret in the CAS service definition.
	ClientSecret string `json:"clientSecret,omitempty"`
	// Scopes contain collection of authorized scopes for this service that act as a filter for the requested scopes in
	// the authorization request
	Scopes *StringCollection `json:"scopes,omitempty"`
	// SupportedGrantTypes contain an optional collection of supported grant types for this service.
	//
	// See also https://apereo.github.io/cas/7.3.x/authentication/OAuth-Authentication-Clients-ResponsesGrants.html
	SupportedGrantTypes *StringCollection `json:"supportedGrantTypes,omitempty"`
	// SupportedResponseTypes contain an optional collection of supported response types for this service.
	//
	// See also https://apereo.github.io/cas/7.3.x/authentication/OAuth-Authentication-Clients-ResponsesGrants.html
	SupportedResponseTypes *StringCollection `json:"supportedResponseTypes,omitempty"`
	// Audience contains a collection of values that can control the aud field in JWT access tokens or ID tokens.
	// If left undefined, the client ID will typically be used instead
	Audience *StringCollection `json:"audience,omitempty"`
}

type RegisteredServiceProperties struct {
	Class   string
	Entries map[string]RegisteredServiceProperty
}

func NewRegisteredServiceProperties() *RegisteredServiceProperties {
	return &RegisteredServiceProperties{
		Class:   hashMapClass,
		Entries: map[string]RegisteredServiceProperty{},
	}
}

func (p RegisteredServiceProperties) MarshalJSON() ([]byte, error) {
	payload := map[string]any{
		classJSONKey: p.Class,
	}
	for key, value := range p.Entries {
		payload[key] = value
	}
	return json.Marshal(payload)
}

func (p *RegisteredServiceProperties) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	props := &RegisteredServiceProperties{
		Class:   hashMapClass,
		Entries: map[string]RegisteredServiceProperty{},
	}

	if class, ok := raw[classJSONKey]; ok {
		if err := json.Unmarshal(class, &props.Class); err != nil {
			return err
		}
		delete(raw, classJSONKey)
	}

	for key, value := range raw {
		var property RegisteredServiceProperty
		if err := json.Unmarshal(value, &property); err != nil {
			return fmt.Errorf("failed to unmarshal property %q: %w", key, err)
		}
		props.Entries[key] = property
	}

	*p = *props
	return nil
}

func (p *RegisteredServiceProperties) GetFirstValue(key string) string {
	if p == nil {
		return ""
	}

	property, ok := p.Entries[key]
	if !ok || len(property.Values.Values) == 0 {
		return ""
	}

	return property.Values.Values[0]
}

type RegisteredServiceProperty struct {
	Class  string           `json:"@class"`
	Values StringCollection `json:"values"`
}

func NewRegisteredServiceProperty(values ...string) RegisteredServiceProperty {
	return RegisteredServiceProperty{
		Class:  servicePropertyClass,
		Values: NewStringCollection(linkedHashSetClass, values...),
	}
}

// StringCollection associates a given slice of values to a Java collection class
type StringCollection struct {
	Class  string
	Values []string
}

func NewStringCollection(class string, values ...string) StringCollection {
	return StringCollection{
		Class:  class,
		Values: values,
	}
}

func (c StringCollection) MarshalJSON() ([]byte, error) {
	if c.Class == "" {
		c.Class = hashSetClass
	}
	return json.Marshal([]any{c.Class, c.Values})
}

func (c *StringCollection) UnmarshalJSON(data []byte) error {
	// Accept null to mirror omitted optional CAS collection fields.
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*c = StringCollection{}
		return nil
	}

	// Prefer the CAS Java collection wrapper before falling back to a plain string array.
	if wrapper, ok, err := unmarshalStringCollectionWrapper(data); err != nil {
		return err
	} else if ok {
		*c = wrapper
		return nil
	}

	var values []string
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}

	*c = StringCollection{Class: hashSetClass, Values: values}
	return nil
}

type registeredServicesList []RegisteredService

func (l *registeredServicesList) UnmarshalJSON(data []byte) error {
	// Accept null to mirror empty responses from the CAS API.
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*l = nil
		return nil
	}

	// Prefer the CAS Java collection wrapper before trying object or array payloads.
	if services, ok, err := unmarshalRegisteredServicesWrapper(data); err != nil {
		return err
	} else if ok {
		*l = services
		return nil
	}

	type response struct {
		Services []RegisteredService `json:"services"`
	}
	var objectResponse response
	if err := json.Unmarshal(data, &objectResponse); err == nil && objectResponse.Services != nil {
		*l = objectResponse.Services
		return nil
	}

	var direct []RegisteredService
	if err := json.Unmarshal(data, &direct); err != nil {
		return err
	}

	*l = direct
	return nil
}

func unmarshalStringCollectionWrapper(data []byte) (StringCollection, bool, error) {
	var wrapper []json.RawMessage
	// CAS encodes Java collections as [className, values].
	if err := json.Unmarshal(data, &wrapper); err != nil || len(wrapper) != javaCollectionWrapperLength {
		return StringCollection{}, false, nil
	}

	var class string
	if err := json.Unmarshal(wrapper[0], &class); err != nil {
		return StringCollection{}, false, nil
	}

	var values []string
	if err := json.Unmarshal(wrapper[1], &values); err != nil {
		return StringCollection{}, true, err
	}

	return StringCollection{Class: class, Values: values}, true, nil
}

func unmarshalRegisteredServicesWrapper(data []byte) ([]RegisteredService, bool, error) {
	var wrapper []json.RawMessage
	// CAS encodes Java collections as [className, values].
	if err := json.Unmarshal(data, &wrapper); err != nil || len(wrapper) != javaCollectionWrapperLength {
		return nil, false, nil
	}

	var class string
	if err := json.Unmarshal(wrapper[0], &class); err != nil {
		return nil, false, nil
	}

	var services []RegisteredService
	if err := json.Unmarshal(wrapper[1], &services); err != nil {
		return nil, true, err
	}

	return services, true, nil
}
