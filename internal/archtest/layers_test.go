// Package archtest enforces the classic DDD layering of every bounded context (ADR-0005):
//
//	interfaces → application → domain ← infrastructure
//
// The domain depends on nothing but the standard library (and pure libraries); the application
// layer on its own domain and the outbox/tenant platform; interfaces (HTTP) never reach into
// infrastructure; contexts meet only in infrastructure (anti-corruption layers) and in the
// composition root (internal/server).
package archtest

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const module = "github.com/bakhod1r/kyber/internal/"

// pure libraries the domain may use: no I/O.
var pureLibs = []string{"github.com/bakhod1r/guard/access/domain", "github.com/bakhod1r/emailx"}

type pkgImport struct{ file, from, to string }

func imports(t *testing.T) []pkgImport {
	t.Helper()
	var out []pkgImport
	err := filepath.WalkDir("..", func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		from := filepath.ToSlash(filepath.Dir(strings.TrimPrefix(path, "../")))
		for _, im := range f.Imports {
			to, _ := strconv.Unquote(im.Path.Value)
			out = append(out, pkgImport{file: path, from: from, to: to})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// layerOf splits "issue/application/x" into ("issue", "application").
func layerOf(p string) (context, layer string) {
	parts := strings.Split(p, "/")
	if len(parts) < 2 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

func stdlib(path string) bool { return !strings.Contains(strings.SplitN(path, "/", 2)[0], ".") }

func TestLayers(t *testing.T) {
	list := imports(t)
	if len(list) < 100 {
		t.Fatalf("only %d imports found: is the walk rooted at internal/?", len(list))
	}
	for _, im := range list {
		ctx, layer := layerOf(im.from)
		if ctx == "platform" || ctx == "server" || ctx == "archtest" || strings.HasSuffix(im.from, "/repotest") {
			continue // repotest: shared contract tests for repositories, test support only
		}
		internal := strings.HasPrefix(im.to, module)
		var toCtx, toLayer string
		if internal {
			toCtx, toLayer = layerOf(strings.TrimPrefix(im.to, module))
		}
		bad := func(why string) { t.Errorf("%s imports %s: %s", im.file, im.to, why) }
		switch layer {
		case "domain":
			switch {
			case internal && (toCtx != ctx || toLayer != "domain"):
				bad("the domain depends only on its own domain")
			case !internal && !stdlib(im.to) && !pure(im.to):
				bad("the domain uses only the standard library and pure libraries")
			}
		case "application":
			switch {
			case internal && toCtx == "platform" && toLayer != "outbox" && toLayer != "tenant":
				bad("the application layer may use only platform/outbox and platform/tenant")
			case internal && toCtx != "platform" && (toCtx != ctx || toLayer != "domain"):
				bad("the application layer depends only on its own domain (other contexts through ports)")
			case !internal && !stdlib(im.to) && !pure(im.to):
				bad("the application layer uses only the standard library and pure libraries")
			}
		case "interfaces":
			if internal && toCtx != "platform" && (toCtx != ctx || (toLayer != "application" && toLayer != "domain")) {
				bad("interfaces depend only on their own application and domain layers")
			}
		case "infrastructure":
			if internal && toLayer == "interfaces" {
				bad("infrastructure never depends on interfaces")
			}
		default:
			t.Errorf("%s: package outside the four layers (domain, application, infrastructure, interfaces)", im.from)
		}
	}
}

func pure(path string) bool {
	for _, p := range pureLibs {
		if path == p || strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return false
}
