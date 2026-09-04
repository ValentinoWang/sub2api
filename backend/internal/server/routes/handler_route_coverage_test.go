package routes

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Handlers wired into handler.Handlers but reached through a dedicated registration
// function that takes them as a parameter rather than through the shared `h` value.
var handlersRoutedByParameter = map[string]string{
	"Payment":        "RegisterPaymentRoutes takes *handler.PaymentHandler directly",
	"PaymentWebhook": "RegisterPaymentRoutes takes *handler.PaymentWebhookHandler directly",
}

// Handlers whose routes live outside this package.
var handlersRoutedElsewhere = map[string]string{
	"Admin": "admin sub-handlers are registered from admin.go via h.Admin.*",
}

// TestEveryHandlerIsReachableFromSomeRoute guards a wiring gap that is silent at compile
// time: a handler can be constructed, added to handler.Handlers and injected by wire while
// no route ever references it, so the endpoint 404s in production and nothing fails to build.
func TestEveryHandlerIsReachableFromSomeRoute(t *testing.T) {
	fields := handlersStructFields(t)
	require.NotEmpty(t, fields, "failed to parse handler.Handlers fields")

	routeSource := allRouteSource(t)

	unreachable := make([]string, 0)
	for _, field := range fields {
		if reason, ok := handlersRoutedByParameter[field]; ok {
			require.Contains(t, routeSource, "handler."+field+"Handler",
				"%s is documented as routed by parameter (%s) but no route file references that type", field, reason)
			continue
		}
		if _, ok := handlersRoutedElsewhere[field]; ok {
			continue
		}
		if !strings.Contains(routeSource, "h."+field+".") {
			unreachable = append(unreachable, field)
		}
	}
	sort.Strings(unreachable)
	require.Emptyf(t, unreachable,
		"handler.Handlers fields with no route referencing them: %v\n"+
			"Either register a route for each, or document it in handlersRoutedByParameter / handlersRoutedElsewhere.",
		unreachable)
}

// TestPublicStatusRouteIsPubliclyReachable pins the exact shape the public status page needs:
// an unauthenticated GET under /public that is rate limited like the other anonymous endpoints.
func TestPublicStatusRouteIsPubliclyReachable(t *testing.T) {
	source, err := os.ReadFile("auth.go")
	require.NoError(t, err)
	text := string(source)

	require.Contains(t, text, `publicInfo.GET("/status", h.PublicStatus.Get)`,
		"the public status endpoint must stay registered; without it /api/v1/public/status 404s")
	require.Contains(t, text, `publicInfo.Use(panelRateLimiter.PublicIP())`,
		"the public status group must keep anonymous IP rate limiting")
	require.NotContains(t, text, `publicInfo.Use(gin.HandlerFunc(jwtAuth))`,
		"the public status group must stay unauthenticated")
}

func handlersStructFields(t *testing.T) []string {
	t.Helper()
	path := filepath.Join("..", "..", "handler", "handler.go")
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, path, nil, 0)
	require.NoError(t, err)

	var fields []string
	ast.Inspect(parsed, func(node ast.Node) bool {
		spec, ok := node.(*ast.TypeSpec)
		if !ok || spec.Name.Name != "Handlers" {
			return true
		}
		structType, ok := spec.Type.(*ast.StructType)
		if !ok {
			return false
		}
		for _, field := range structType.Fields.List {
			for _, name := range field.Names {
				fields = append(fields, name.Name)
			}
		}
		return false
	})
	return fields
}

func allRouteSource(t *testing.T) string {
	t.Helper()
	entries, err := filepath.Glob("*.go")
	require.NoError(t, err)

	var builder strings.Builder
	for _, entry := range entries {
		if strings.HasSuffix(entry, "_test.go") {
			continue
		}
		content, err := os.ReadFile(entry)
		require.NoError(t, err)
		builder.Write(content)
	}
	return builder.String()
}
