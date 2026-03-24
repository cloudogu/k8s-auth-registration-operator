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

type RegistrationData map[string][]byte

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

func (rr RegistrationResult) GetRegistrationData() RegistrationData {
	if rr.Protocol == ProtocolCAS && rr.CAS != nil {
		return RegistrationData{"cas_client_id": []byte(rr.CAS.ServiceID)}
	}

	if rr.Protocol == ProtocolOIDC && rr.OIDC != nil {
		return RegistrationData{
			"oidc_client_id":     []byte(rr.OIDC.ClientID),
			"oidc_client_secret": []byte(rr.OIDC.ClientSecret),
			"oidc_issuer_url":    []byte(rr.OIDC.IssuerURL),
		}
	}

	if rr.Protocol == ProtocolOAuth && rr.OAuth != nil {
		return RegistrationData{
			"oauth":               []byte(rr.OAuth.ClientID),
			"oauth_client_secret": []byte(rr.OAuth.ClientSecret),
			"oauth_auth_url":      []byte(rr.OAuth.AuthURL),
			"oauth_token_url":     []byte(rr.OAuth.TokenURL),
		}
	}

	return RegistrationData{}
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
