package domain

import (
	authregistrationv1 "github.com/cloudogu/k8s-auth-registration-lib/api/v1"
)

// Protocol is the protocol used for the registration.
type Protocol string

const (
	ProtocolCAS   Protocol = "CAS"
	ProtocolOIDC  Protocol = "OIDC"
	ProtocolOAuth Protocol = "OAUTH"
)

type Registration struct {
	Protocol  Protocol
	Consumer  string
	LogoutURL string
	Params    map[string]string
}

type OIDCResult struct {
	ClientID     string
	ClientSecret string
	IssuerURL    string
}

type OAuthResult struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
}

type CASResult struct {
	ServiceID string
}

type RegistrationResult struct {
	Protocol       Protocol
	RegistrationID string

	// Protokollspezifische, typisierte Details
	OIDC  *OIDCResult
	OAuth *OAuthResult
	CAS   *CASResult
}

func (rr RegistrationResult) GetSecretData() map[string][]byte {
	if rr.Protocol == ProtocolCAS && rr.CAS != nil {
		return map[string][]byte{"serviceId": []byte(rr.CAS.ServiceID)}
	}

	if rr.Protocol == ProtocolOIDC && rr.OIDC != nil {
		return map[string][]byte{
			"clientId":     []byte(rr.OIDC.ClientID),
			"clientSecret": []byte(rr.OIDC.ClientSecret),
			"issuerUrl":    []byte(rr.OIDC.IssuerURL),
		}
	}

	if rr.Protocol == ProtocolOAuth && rr.OAuth != nil {
		return map[string][]byte{
			"clientId":     []byte(rr.OAuth.ClientID),
			"clientSecret": []byte(rr.OAuth.ClientSecret),
			"authURL":      []byte(rr.OAuth.AuthURL),
			"tokenURL":     []byte(rr.OAuth.TokenURL),
		}
	}

	return map[string][]byte{}
}

func FromAuthRegistration(registration *authregistrationv1.AuthRegistration) Registration {
	logoutURL := ""
	if registration.Spec.LogoutURL != nil {
		logoutURL = *registration.Spec.LogoutURL
	}

	return Registration{
		Protocol:  Protocol(registration.Spec.Protocol),
		Consumer:  registration.Spec.Consumer,
		Params:    registration.Spec.Params,
		LogoutURL: logoutURL,
	}
}
