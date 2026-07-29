package wrap

import (
	"strings"
	"testing"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/cmd"
	gofrConfig "gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
)

// createTestContext creates a test gofr.Context for CMD applications.
func createTestContext() *gofr.Context {
	c := container.NewContainer(gofrConfig.NewEnvFile("", logging.NewMockLogger(logging.DEBUG)))
	req := cmd.NewRequest([]string{})

	return &gofr.Context{
		Context:   req.Context(),
		Request:   req,
		Container: c,
	}
}

// The request-wrapper template ranges over []ServiceRequest, so it must render
// the request type's Name, not the whole struct. Regression test for #75:
// `{{ $request }}` printed the struct as `{GetThingRequest GetThingRequest}`,
// producing `type {GetThingRequest GetThingRequest}Wrapper` that would not
// compile.
func TestGenerateGoFrRequestWrapper_UsesRequestTypeName(t *testing.T) {
	out := generateGoFrRequestWrapper(createTestContext(), &WrapperData{
		Package: "example",
		Source:  "example.proto",
		Requests: []ServiceRequest{
			{Request: "GetThingRequest"},
			{Request: "CreateThingRequest"},
		},
	})

	// Every request type renders correctly, for both entries.
	want := []string{
		"type GetThingRequestWrapper struct {",
		"*GetThingRequest",
		"func (h *GetThingRequestWrapper) Context() context.Context {",
		"func (h *GetThingRequestWrapper) Bind(p interface{}) error {",
		"reflect.ValueOf(h.GetThingRequest).Elem()",
		"type CreateThingRequestWrapper struct {",
	}
	for _, s := range want {
		if !strings.Contains(out, s) {
			t.Errorf("generated request wrapper missing %q\n---\n%s", s, out)
		}
	}

	// The Go struct-literal formatting must not leak into the output.
	if strings.Contains(out, "{GetThingRequest") || strings.Contains(out, "{CreateThingRequest") {
		t.Errorf("request wrapper leaked a struct literal into the output:\n%s", out)
	}
}
