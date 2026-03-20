package cas

import (
	"bytes"
	"encoding/json"
	"fmt"
)

const (
	casRegisteredServiceClass   = "org.apereo.cas.services.CasRegisteredService"
	oidcRegisteredServiceClass  = "org.apereo.cas.services.OidcRegisteredService"
	oauthRegisteredServiceClass = "org.apereo.cas.support.oauth.services.OAuthRegisteredService"
	hashMapClass                = "java.util.HashMap"
	hashSetClass                = "java.util.HashSet"
	linkedHashSetClass          = "java.util.LinkedHashSet"
	servicePropertyClass        = "org.apereo.cas.services.DefaultRegisteredServiceProperty"
)

type RegisteredService struct {
	Class        string `json:"@class"`
	ID           int64  `json:"id,omitempty"`
	TemplateName string `json:"templateName,omitempty"`
	// Name is required for services managed via the CAS registeredServices API.
	Name string `json:"name,omitempty"`
	// ServiceID is required for services managed via the CAS registeredServices API.
	ServiceID              string                       `json:"serviceId,omitempty"`
	LogoutURL              string                       `json:"logoutUrl,omitempty"`
	Properties             *RegisteredServiceProperties `json:"properties,omitempty"`
	ClientID               string                       `json:"clientId,omitempty"`
	ClientSecret           string                       `json:"clientSecret,omitempty"`
	Scopes                 *StringCollection            `json:"scopes,omitempty"`
	SupportedGrantTypes    *StringCollection            `json:"supportedGrantTypes,omitempty"`
	SupportedResponseTypes *StringCollection            `json:"supportedResponseTypes,omitempty"`
	Audience               *StringCollection            `json:"audience,omitempty"`
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
		"@class": p.Class,
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

	if class, ok := raw["@class"]; ok {
		if err := json.Unmarshal(class, &props.Class); err != nil {
			return err
		}
		delete(raw, "@class")
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
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*c = StringCollection{}
		return nil
	}

	var wrapper []json.RawMessage
	if err := json.Unmarshal(data, &wrapper); err == nil && len(wrapper) == 2 {
		var class string
		if err := json.Unmarshal(wrapper[0], &class); err == nil {
			var values []string
			if err := json.Unmarshal(wrapper[1], &values); err != nil {
				return err
			}
			*c = StringCollection{Class: class, Values: values}
			return nil
		}
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
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*l = nil
		return nil
	}

	var wrapper []json.RawMessage
	if err := json.Unmarshal(data, &wrapper); err == nil && len(wrapper) == 2 {
		var class string
		if err := json.Unmarshal(wrapper[0], &class); err == nil {
			return json.Unmarshal(wrapper[1], (*[]RegisteredService)(l))
		}
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
