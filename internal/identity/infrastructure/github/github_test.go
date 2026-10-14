package github_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/identity/application"
	"github.com/bakhod1r/kyber/internal/identity/domain"
	"github.com/bakhod1r/kyber/internal/identity/infrastructure/github"
)

// fakeGitHub serves the OAuth web flow (github.com) and the REST API (api.github.com).
type fakeGitHub struct {
	*httptest.Server
	user   any // GET /user body
	emails any // GET /user/emails body
	status map[string]int
}

func newFake(t *testing.T) *fakeGitHub {
	t.Helper()
	f := &fakeGitHub{status: map[string]int{},
		user: map[string]any{"id": 583231, "login": "octocat", "name": "The Octocat"},
		emails: []map[string]any{
			{"email": "old@x.uz", "primary": false, "verified": true},
			{"email": "octo@x.uz", "primary": true, "verified": true},
		}}
	mux := http.NewServeMux()
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	mux.HandleFunc("POST /login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("code") != "good-code" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"error":"bad_verification_code"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"gho_x","token_type":"bearer","scope":"read:user,user:email"}`))
	})
	api := func(path string, body func() any) {
		mux.HandleFunc("GET /api"+path, func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer gho_x" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if s := f.status[path]; s != 0 {
				w.WriteHeader(s)
				return
			}
			_ = json.NewEncoder(w).Encode(body())
		})
	}
	api("/user", func() any { return f.user })
	api("/user/emails", func() any { return f.emails })
	return f
}

func client(f *fakeGitHub) *github.Client {
	return github.New(github.Config{ClientID: "cid", ClientSecret: "sec", RedirectURL: "https://kyber.test/cb",
		WebURL: f.URL, APIURL: f.URL + "/api"})
}

func TestAuthURL(t *testing.T) {
	f := newFake(t)
	flow := application.NewOIDCFlow()
	u, err := client(f).AuthURL(context.Background(), flow)
	if err != nil {
		t.Fatal(err)
	}
	q, _ := url.Parse(u)
	v := q.Query()
	if !strings.HasPrefix(u, f.URL+"/login/oauth/authorize?") || v.Get("state") != flow.State || v.Get("client_id") != "cid" ||
		v.Get("redirect_uri") != "https://kyber.test/cb" || !strings.Contains(v.Get("scope"), "user:email") {
		t.Fatalf("auth url = %s", u)
	}
}

func TestExchange(t *testing.T) {
	f := newFake(t)
	p, err := client(f).Exchange(context.Background(), "good-code", application.NewOIDCFlow())
	if err != nil {
		t.Fatal(err)
	}
	want := application.ExternalProfile{Provider: domain.ProviderGitHub, Subject: "583231", Email: "octo@x.uz", EmailVerified: true, Name: "The Octocat"}
	if p != want {
		t.Fatalf("profile = %+v", p)
	}
	// No public name: the login is used. No primary verified email: no email is vouched for.
	f.user = map[string]any{"id": 7, "login": "ghost", "name": nil}
	f.emails = []map[string]any{{"email": "u@x.uz", "primary": true, "verified": false}}
	p, err = client(f).Exchange(context.Background(), "good-code", application.NewOIDCFlow())
	if err != nil || p.Subject != "7" || p.Name != "ghost" || p.Email != "" || p.EmailVerified {
		t.Fatalf("profile = %+v %v", p, err)
	}
}

func TestExchangeRejects(t *testing.T) {
	cases := map[string]func(f *fakeGitHub) string{
		"bad code":        func(*fakeGitHub) string { return "bad" },
		"user fails":      func(f *fakeGitHub) string { f.status["/user"] = 500; return "good-code" },
		"emails fail":     func(f *fakeGitHub) string { f.status["/user/emails"] = 403; return "good-code" },
		"malformed user":  func(f *fakeGitHub) string { f.user = "nope"; return "good-code" },
		"no user id":      func(f *fakeGitHub) string { f.user = map[string]any{"login": "x"}; return "good-code" },
		"malformed email": func(f *fakeGitHub) string { f.emails = 1; return "good-code" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFake(t)
			code := mutate(f)
			if _, err := client(f).Exchange(context.Background(), code, application.NewOIDCFlow()); !errors.Is(err, github.ErrInvalidLogin) {
				t.Fatalf("err = %v", err)
			}
		})
	}
	down := github.New(github.Config{ClientID: "c", ClientSecret: "s", WebURL: "http://127.0.0.1:1", APIURL: "http://127.0.0.1:1"})
	if _, err := down.Exchange(context.Background(), "x", application.NewOIDCFlow()); err == nil {
		t.Fatal("unreachable GitHub must fail")
	}
}

func TestAPIUnreachableAndBadURL(t *testing.T) {
	f := newFake(t)
	c := github.New(github.Config{ClientID: "c", ClientSecret: "s", WebURL: f.URL, APIURL: "http://127.0.0.1:1"})
	if _, err := c.Exchange(context.Background(), "good-code", application.NewOIDCFlow()); !errors.Is(err, github.ErrInvalidLogin) {
		t.Fatalf("err = %v", err)
	}
	c = github.New(github.Config{ClientID: "c", ClientSecret: "s", WebURL: f.URL, APIURL: "http://bad host"})
	if _, err := c.Exchange(context.Background(), "good-code", application.NewOIDCFlow()); !errors.Is(err, github.ErrInvalidLogin) {
		t.Fatalf("err = %v", err)
	}
}

func TestConfig(t *testing.T) {
	if github.New(github.Config{}).Enabled() || !github.New(github.Config{ClientID: "c", ClientSecret: "s"}).Enabled() {
		t.Fatal("enabled only with client id and secret")
	}
	u, _ := github.New(github.Config{ClientID: "c", ClientSecret: "s"}).AuthURL(context.Background(), application.NewOIDCFlow())
	if !strings.HasPrefix(u, "https://github.com/login/oauth/authorize?") {
		t.Fatalf("default auth url = %s", u)
	}
}
