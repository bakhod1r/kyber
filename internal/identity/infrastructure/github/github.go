// Package github signs users in with GitHub (OAuth web flow with state and PKCE). GitHub is
// not an OpenID provider: the profile comes from its REST API (GET /user and GET /user/emails)
// called with the user's access token. Only the primary, verified email is vouched for.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"golang.org/x/oauth2"

	"github.com/bakhod1r/kyber/internal/identity/application"
	"github.com/bakhod1r/kyber/internal/identity/domain"
)

// ErrInvalidLogin is the application layer's error for a failed external sign-in.
var ErrInvalidLogin = application.ErrInvalidLogin

const (
	WebURL = "https://github.com"
	APIURL = "https://api.github.com"
)

type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string // e.g. https://kyber.example.com/api/v1/auth/github/callback
	WebURL       string // default WebURL; tests point it at a fake
	APIURL       string // default APIURL
}

type Client struct {
	oauth *oauth2.Config
	api   string
}

func New(cfg Config) *Client {
	if cfg.WebURL == "" {
		cfg.WebURL = WebURL
	}
	if cfg.APIURL == "" {
		cfg.APIURL = APIURL
	}
	return &Client{api: cfg.APIURL, oauth: &oauth2.Config{ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret,
		RedirectURL: cfg.RedirectURL, Scopes: []string{"read:user", "user:email"},
		Endpoint: oauth2.Endpoint{AuthURL: cfg.WebURL + "/login/oauth/authorize", TokenURL: cfg.WebURL + "/login/oauth/access_token",
			AuthStyle: oauth2.AuthStyleInParams}}}
}

func (c *Client) Enabled() bool { return c.oauth.ClientID != "" && c.oauth.ClientSecret != "" }

// AuthURL is where the browser is sent to authorize Kyber. GitHub ignores the nonce.
func (c *Client) AuthURL(_ context.Context, f application.OIDCFlow) (string, error) {
	return c.oauth.AuthCodeURL(f.State, oauth2.S256ChallengeOption(f.Verifier)), nil
}

// Exchange redeems the code and reads the user's profile and primary verified email.
func (c *Client) Exchange(ctx context.Context, code string, f application.OIDCFlow) (application.ExternalProfile, error) {
	tok, err := c.oauth.Exchange(ctx, code, oauth2.VerifierOption(f.Verifier))
	if err != nil {
		return application.ExternalProfile{}, fmt.Errorf("%w: code exchange: %w", ErrInvalidLogin, err)
	}
	var user struct {
		ID    int64   `json:"id"`
		Login string  `json:"login"`
		Name  *string `json:"name"`
	}
	if err := c.get(ctx, tok.AccessToken, "/user", &user); err != nil {
		return application.ExternalProfile{}, err
	}
	if user.ID == 0 {
		return application.ExternalProfile{}, fmt.Errorf("%w: github user without id", ErrInvalidLogin)
	}
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := c.get(ctx, tok.AccessToken, "/user/emails", &emails); err != nil {
		return application.ExternalProfile{}, err
	}
	p := application.ExternalProfile{Provider: domain.ProviderGitHub, Subject: strconv.FormatInt(user.ID, 10), Name: user.Login}
	if user.Name != nil && *user.Name != "" {
		p.Name = *user.Name
	}
	for _, e := range emails {
		if e.Primary && e.Verified {
			p.Email, p.EmailVerified = e.Email, true
		}
	}
	return p, nil
}

func (c *Client) get(ctx context.Context, token, path string, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.api+path, nil)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidLogin, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: github %s: %w", ErrInvalidLogin, path, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: github %s: %s", ErrInvalidLogin, path, res.Status)
	}
	if err := json.NewDecoder(res.Body).Decode(into); err != nil {
		return fmt.Errorf("%w: github %s: %w", ErrInvalidLogin, path, err)
	}
	return nil
}
