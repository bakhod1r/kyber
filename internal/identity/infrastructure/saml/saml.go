// Package saml is the SAML 2.0 service provider adapter (ADR-0017): SP-initiated sign-in with the
// HTTP-Redirect binding, responses on the HTTP-POST binding, signatures checked against the
// identity provider's metadata (crewjam/saml).
package saml

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	crewjam "github.com/crewjam/saml"

	"github.com/bakhod1r/kyber/internal/identity/application"
	"github.com/bakhod1r/kyber/internal/identity/domain"
)

// Config is the service provider's identity and its identity provider.
type Config struct {
	BaseURL     string // Kyber's external URL (KYBER_PUBLIC_URL)
	IDPMetadata []byte // the identity provider's metadata XML
	Key         *rsa.PrivateKey
	Cert        *x509.Certificate
}

// SP signs users in through one SAML identity provider.
type SP struct{ sp crewjam.ServiceProvider }

// Paths of the service provider's endpoints.
const (
	MetadataPath = "/api/v1/auth/saml/metadata"
	ACSPath      = "/api/v1/auth/saml/acs"
)

func New(c Config) (*SP, error) {
	var md crewjam.EntityDescriptor
	if err := xml.Unmarshal(c.IDPMetadata, &md); err != nil {
		return nil, fmt.Errorf("saml: identity provider metadata: %w", err)
	}
	base := strings.TrimRight(c.BaseURL, "/")
	meta, err := url.Parse(base + MetadataPath)
	if err != nil {
		return nil, err
	}
	acs, _ := url.Parse(base + ACSPath)
	return &SP{sp: crewjam.ServiceProvider{
		EntityID: meta.String(), Key: c.Key, Certificate: c.Cert, MetadataURL: *meta, AcsURL: *acs,
		IDPMetadata: &md, AuthnNameIDFormat: crewjam.EmailAddressNameIDFormat,
	}}, nil
}

func (s *SP) Enabled() bool { return s != nil }

// Metadata is the service provider's metadata XML for the identity provider's configuration.
func (s *SP) Metadata() ([]byte, error) { return xml.MarshalIndent(s.sp.Metadata(), "", "  ") }

// Start returns where to send the browser and the request id the response must answer.
func (s *SP) Start(relayState string) (redirect, requestID string, err error) {
	req, err := s.sp.MakeAuthenticationRequest(s.sp.GetSSOBindingLocation(crewjam.HTTPRedirectBinding), crewjam.HTTPRedirectBinding, crewjam.HTTPPostBinding)
	if err != nil {
		return "", "", err
	}
	u, err := req.Redirect(relayState, &s.sp)
	if err != nil {
		return "", "", err
	}
	return u.String(), req.ID, nil
}

// ErrNoEmail: the assertion names no email address.
var ErrNoEmail = errors.New("saml: the identity provider sent no email address")

// Profile verifies the POSTed response (signature, audience, destination, validity window and that
// it answers one of requestIDs) and reads the user from it. The identity provider is the
// organisation's own directory, so its email address counts as verified.
func (s *SP) Profile(r *http.Request, requestIDs []string) (application.ExternalProfile, error) {
	if err := r.ParseForm(); err != nil { // crewjam reads PostForm without parsing it
		return application.ExternalProfile{}, fmt.Errorf("%w: %w", application.ErrInvalidLogin, err)
	}
	a, err := s.sp.ParseResponse(r, requestIDs)
	if err != nil {
		var ie *crewjam.InvalidResponseError
		if errors.As(err, &ie) {
			err = fmt.Errorf("%w: %w", application.ErrInvalidLogin, ie.PrivateErr)
		}
		return application.ExternalProfile{}, err
	}
	attrs := map[string]string{}
	for _, st := range a.AttributeStatements {
		for _, at := range st.Attributes {
			if len(at.Values) > 0 {
				name := strings.ToLower(at.Name)
				if i := strings.LastIndexAny(name, "/:"); i >= 0 { // claim URIs: …/emailaddress
					name = name[i+1:]
				}
				attrs[name] = at.Values[0].Value
				if at.FriendlyName != "" { // e.g. Name="urn:oid:2.5.4.3" FriendlyName="cn"
					attrs[strings.ToLower(at.FriendlyName)] = at.Values[0].Value
				}
			}
		}
	}
	var nameID string
	if a.Subject != nil && a.Subject.NameID != nil {
		nameID = a.Subject.NameID.Value
	}
	email := first(attrs["email"], attrs["mail"], attrs["emailaddress"])
	if email == "" && strings.Contains(nameID, "@") {
		email = nameID
	}
	if email == "" {
		return application.ExternalProfile{}, ErrNoEmail
	}
	name := first(attrs["displayname"], attrs["name"], attrs["cn"],
		strings.TrimSpace(first(attrs["givenname"], attrs["firstname"])+" "+first(attrs["sn"], attrs["surname"], attrs["lastname"])), strings.Split(email, "@")[0])
	subject := nameID
	if subject == "" {
		subject = strings.ToLower(email)
	}
	return application.ExternalProfile{Provider: domain.ProviderSAML, Subject: subject, Email: email, EmailVerified: true, Name: name}, nil
}

func first(vs ...string) string {
	for _, v := range vs {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
