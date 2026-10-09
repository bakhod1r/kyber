package server_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

// KYB-T3: every response produced by the acceptance tests is validated against
// api/openapi.yaml, and the suite fails if the spec and the server drift apart
// in either direction (undocumented routes, or documented operations never exercised).

type contract struct {
	once         sync.Once
	doc          *openapi3.T
	router       routers.Router
	err          error
	mu           sync.Mutex
	hit          map[string]bool // operationId -> exercised
	undocumented map[string]bool // "METHOD /path" outside the spec
}

var spec = &contract{hit: map[string]bool{}, undocumented: map[string]bool{}}

// allowedUndocumented are requests tests make on purpose outside the API surface.
var allowedUndocumented = map[string]bool{
	"GET /": true, "GET /login": true, "GET /projects/KYB": true, // SPA routes
	"GET /api/v1/unknown": true, // proves unknown API paths are not served by the SPA
}

func (c *contract) load() error {
	c.once.Do(func() {
		loader := openapi3.NewLoader()
		c.doc, c.err = loader.LoadFromFile("../../api/openapi.yaml")
		if c.err != nil {
			return
		}
		if c.err = c.doc.Validate(context.Background()); c.err != nil {
			return
		}
		c.doc.Servers = nil // match on path only; tests run on random ports
		c.router, c.err = gorillamux.NewRouter(c.doc)
	})
	return c.err
}

// check validates one response and returns a fresh body reader for the caller.
func (c *contract) check(t *testing.T, req *http.Request, res *http.Response) {
	t.Helper()
	if err := c.load(); err != nil {
		t.Fatalf("openapi spec: %v", err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	res.Body = io.NopCloser(bytes.NewReader(body))

	route, params, err := c.router.FindRoute(req)
	if err != nil {
		c.mu.Lock()
		c.undocumented[req.Method+" "+req.URL.Path] = true
		c.mu.Unlock()
		return
	}
	c.mu.Lock()
	c.hit[route.Operation.OperationID] = true
	c.mu.Unlock()

	input := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request: req, PathParams: params, Route: route,
			Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
		},
		Status: res.StatusCode,
		Header: res.Header,
		Body:   io.NopCloser(bytes.NewReader(body)),
		Options: &openapi3filter.Options{
			IncludeResponseStatus: true, // undocumented status codes are failures
			MultiError:            true,
		},
	}
	if err := openapi3filter.ValidateResponse(context.Background(), input); err != nil {
		t.Errorf("contract violation: %s %s -> %d: %v\nbody: %s", req.Method, req.URL.Path, res.StatusCode, err, body)
	}
}

// report returns drift problems once all tests have run.
func (c *contract) report() []string {
	if c.doc == nil {
		return nil
	}
	var problems []string
	for path, item := range c.doc.Paths.Map() {
		for method, op := range item.Operations() {
			if !c.hit[op.OperationID] {
				problems = append(problems, fmt.Sprintf("documented but never exercised: %s %s (%s)", method, path, op.OperationID))
			}
		}
	}
	for r := range c.undocumented {
		if !allowedUndocumented[r] && !strings.HasPrefix(r, "GET /assets/") {
			problems = append(problems, "served but not documented: "+r)
		}
	}
	sort.Strings(problems)
	return problems
}

func TestMain(m *testing.M) {
	code := m.Run()
	if code == 0 && !testing.Short() {
		if problems := spec.report(); len(problems) > 0 && runAll() {
			fmt.Fprintln(os.Stderr, "OpenAPI drift:\n  "+strings.Join(problems, "\n  "))
			code = 1
		}
	}
	os.Exit(code)
}

// runAll reports whether the whole package ran (drift checks are meaningless under -run filters).
func runAll() bool {
	for _, a := range os.Args {
		if strings.HasPrefix(a, "-test.run") {
			return false
		}
	}
	return true
}
