package wrap

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

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

// testWrapperData returns a service with one unary and one server-streaming
// method, enough to exercise every naming path in the templates.
func testWrapperData() *WrapperData {
	return &WrapperData{
		Package: "hello",
		Service: "Hello",
		Source:  "hello.proto",
		Methods: []ServiceMethod{
			{Name: "SayHello", Request: "HelloRequest", Response: "HelloResponse"},
			{Name: "LotsOfReplies", Request: "HelloRequest", Response: "HelloResponse", StreamsResponse: true},
		},
		Requests: []ServiceRequest{{Request: "HelloRequest"}},
	}
}

// The generated GoFr wrapper must name the CLI's own types after the service
// (issue #40): Register<Svc>ServiceWithGofr / <Svc>GoFrService, not ...Server...
// The protoc-gen-go-grpc contract symbols the wrapper references must stay
// exactly as protoc emits them, otherwise the generated code won't compile.
func TestGenerateGoFrServerWrapper_Naming(t *testing.T) {
	out := generateGoFrServerWrapper(createTestContext(), testWrapperData())

	wantService := []string{
		"func NewHelloGoFrService() *HelloGoFrService {",
		"type HelloServiceWithGofr interface {",
		"type HelloServiceWrapper struct {",
		"func RegisterHelloServiceWithGofr(app *gofr.App, srv HelloServiceWithGofr) {",
		"func (h *HelloServiceWrapper) SayHello(",
	}
	for _, s := range wantService {
		assert.Contains(t, out, s, "CLI-owned type must be renamed to Service")
	}

	wantProtoc := []string{
		"mustEmbedUnimplementedHelloServer()", // required method of protoc HelloServer
		"RegisterHelloServer(s, wrapper)",     // protoc registration func
		"Hello_LotsOfRepliesServer",           // protoc stream interface
		"registerServerWithGofr(app, srv,",    // generic CLI helper, not service-scoped
	}
	for _, s := range wantProtoc {
		assert.Contains(t, out, s, "protoc/contract symbol must be left unchanged")
	}

	// The old CLI names must be gone entirely.
	for _, s := range []string{"NewHelloGoFrServer", "HelloServerWithGofr", "HelloServerWrapper", "RegisterHelloServerWithGofr"} {
		assert.NotContains(t, out, s, "old Server-suffixed CLI name must not remain")
	}
}

// The server scaffold users edit must expose the renamed struct + usage hint.
func TestGenerateGoFrServer_Naming(t *testing.T) {
	out := generateGoFrServer(createTestContext(), testWrapperData())

	for _, s := range []string{
		"type HelloGoFrService struct {",
		"func (s *HelloGoFrService) SayHello(",
		"RegisterHelloServiceWithGofr(app,",
		"NewHelloGoFrService()",
	} {
		assert.Contains(t, out, s)
	}

	// Streaming handler still references the protoc stream type.
	assert.Contains(t, out, "Hello_LotsOfRepliesServer")
	assert.NotContains(t, out, "HelloGoFrServer")
}

// The client template is unrelated to the Server->Service rename and must keep
// its GoFrClient / protoc client naming intact.
func TestGenerateGoFrClient_NamingUnaffected(t *testing.T) {
	out := generateGoFrClient(createTestContext(), testWrapperData())

	for _, s := range []string{
		"type HelloGoFrClient interface {",
		"type HelloClientWrapper struct {",
		"func NewHelloGoFrClient(",
		"NewHelloClient(conn)", // protoc client constructor
	} {
		assert.Contains(t, out, s)
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
