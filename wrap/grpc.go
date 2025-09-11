package wrap

import (
	"bytes"
	"errors"
	"os"
	"path"
	"strings"
	"text/template"

	"github.com/emicklei/proto"
	"gofr.dev/pkg/gofr"
)

const (
	filePerm                = 0644
	serverFileSuffix        = "_server.go"
	serverWrapperFileSuffix = "_gofr.go"
	clientFileSuffix        = "_client.go"
	clientHealthFile        = "health_client.go"
	serverHealthFile        = "health_gofr.go"
	serverRequestFile       = "request_gofr.go"
)

var (
	ErrNoProtoFile        = errors.New("proto file path is required")
	ErrOpeningProtoFile   = errors.New("error opening the proto file")
	ErrFailedToParseProto = errors.New("failed to parse proto file")
	ErrGeneratingWrapper  = errors.New("error while generating the code using proto file")
	ErrWritingFile        = errors.New("error writing the generated code to the file")
)

// ServiceMethod represents a method in a proto service.
type ServiceMethod struct {
	Name            string
	Request         string
	Response        string
	StreamsRequest  bool
	StreamsResponse bool
}

// ProtoService represents a service in a proto file.
type ProtoService struct {
	Name    string
	Methods []ServiceMethod
}

// WrapperData is the template data structure.
type WrapperData struct {
	Package  string
	Service  string
	Methods  []ServiceMethod
	Requests []string
	Source   string
}

type FileType struct {
	FileSuffix    string
	CodeGenerator func(*gofr.Context, *WrapperData) string
}

// BuildGRPCGoFrClient generates gRPC client wrapper code based on a proto definition.
func BuildGRPCGoFrClient(ctx *gofr.Context) (any, error) {
	gRPCClient := []FileType{
		{FileSuffix: clientFileSuffix, CodeGenerator: generateGoFrClient},
		{FileSuffix: clientHealthFile, CodeGenerator: generateGoFrClientHealth},
	}

	return generateWrapper(ctx, gRPCClient...)
}

// BuildGRPCGoFrServer generates gRPC client and server code based on a proto definition.
func BuildGRPCGoFrServer(ctx *gofr.Context) (any, error) {
	gRPCServer := []FileType{
		{FileSuffix: serverWrapperFileSuffix, CodeGenerator: generateGoFrServerWrapper},
		{FileSuffix: serverHealthFile, CodeGenerator: generateGoFrServerHealthWrapper},
		{FileSuffix: serverRequestFile, CodeGenerator: generateGoFrRequestWrapper},
		{FileSuffix: serverFileSuffix, CodeGenerator: generateGoFrServer},
	}

	return generateWrapper(ctx, gRPCServer...)
}

// generateWrapper executes the function for specified FileType to create GoFr integrated
// gRPC server/client files with the required services in proto file and
// specified suffix for every service specified in the proto file.
func generateWrapper(ctx *gofr.Context, options ...FileType) (any, error) {
	protoPath := ctx.Param("proto")
	if protoPath == "" {
		ctx.Logger.Error(ErrNoProtoFile)
		return nil, ErrNoProtoFile
	}

	definition, err := parseProtoFile(ctx, protoPath)
	if err != nil {
		ctx.Logger.Errorf("Failed to parse proto file: %v", err)
		return nil, err
	}

	projectPath, packageName := getPackageAndProject(ctx, definition, protoPath)
	services := getServices(ctx, definition)
	requests := getRequests(ctx, services)

	for _, service := range services {
		wrapperData := WrapperData{
			Package:  packageName,
			Service:  service.Name,
			Methods:  service.Methods,
			Requests: uniqueRequestTypes(ctx, service.Methods),
			Source:   path.Base(protoPath),
		}

		if err := generateFiles(ctx, projectPath, service.Name, &wrapperData, requests, options...); err != nil {
			return nil, err
		}
	}

	ctx.Logger.Info("Successfully generated all files for GoFr integrated gRPC servers/clients")

	return "Successfully generated all files for GoFr integrated gRPC servers/clients", nil
}

// parseProtoFile opens and parses the proto file.
func parseProtoFile(ctx *gofr.Context, protoPath string) (*proto.Proto, error) {
	file, err := os.Open(protoPath)
	if err != nil {
		ctx.Logger.Errorf("Failed to open proto file: %v", err)
		return nil, ErrOpeningProtoFile
	}
	defer file.Close()

	parser := proto.NewParser(file)

	definition, err := parser.Parse()
	if err != nil {
		ctx.Logger.Errorf("Failed to parse proto file: %v", err)
		return nil, ErrFailedToParseProto
	}

	return definition, nil
}

// generateFiles generates files for a given service.
func generateFiles(ctx *gofr.Context, projectPath, serviceName string, wrapperData *WrapperData,
	requests []string, options ...FileType) error {
	for _, option := range options {
		if option.FileSuffix == serverRequestFile {
			wrapperData.Requests = requests
		}

		generatedCode := option.CodeGenerator(ctx, wrapperData)
		if generatedCode == "" {
			ctx.Logger.Errorf("Failed to generate code for service %s with file suffix %s", serviceName, option.FileSuffix)
			return ErrGeneratingWrapper
		}

		outputFilePath := getOutputFilePath(projectPath, serviceName, option.FileSuffix)
		if err := os.WriteFile(outputFilePath, []byte(generatedCode), filePerm); err != nil {
			ctx.Logger.Errorf("Failed to write file %s: %v", outputFilePath, err)
			return ErrWritingFile
		}

		ctx.Logger.Infof("Generated file for service %s at %s", serviceName, outputFilePath)
	}

	return nil
}

// getOutputFilePath generates the output file path based on the file suffix.
func getOutputFilePath(projectPath, serviceName, fileSuffix string) string {
	switch fileSuffix {
	case clientHealthFile:
		return path.Join(projectPath, clientHealthFile)
	case serverHealthFile:
		return path.Join(projectPath, serverHealthFile)
	case serverRequestFile:
		return path.Join(projectPath, serverRequestFile)
	default:
		return path.Join(projectPath, strings.ToLower(serviceName)+fileSuffix)
	}
}

// getRequests extracts all unique request types from the services.
func getRequests(ctx *gofr.Context, services []ProtoService) []string {
	requests := make(map[string]bool)

	for _, service := range services {
		for _, method := range service.Methods {
			requests[method.Request] = true
		}
	}

	ctx.Logger.Debugf("Extracted unique request types: %v", requests)

	return mapKeysToSlice(requests)
}

// uniqueRequestTypes extracts unique request types from methods.
func uniqueRequestTypes(ctx *gofr.Context, methods []ServiceMethod) []string {
	requests := make(map[string]bool)

	for _, method := range methods {
		requests[method.Request] = true // Include all request types
	}

	ctx.Logger.Debugf("Extracted unique request types for methods: %v", requests)

	return mapKeysToSlice(requests)
}

// mapKeysToSlice converts a map's keys to a slice.
func mapKeysToSlice(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}

	return keys
}

// executeTemplate executes a template with the provided data.
func executeTemplate(ctx *gofr.Context, data *WrapperData, tmpl string) string {
	funcMap := template.FuncMap{
		"lowerFirst": func(s string) string {
			if s == "" {
				return ""
			}
			return strings.ToLower(s[:1]) + s[1:]
		},
	}

	tmplInstance := template.Must(template.New("template").Funcs(funcMap).Parse(tmpl))

	var buf bytes.Buffer

	if err := tmplInstance.Execute(&buf, data); err != nil {
		ctx.Logger.Errorf("Template execution failed: %v", err)
		return ""
	}

	return buf.String()
}

// Template generators.
func generateGoFrServerWrapper(ctx *gofr.Context, data *WrapperData) string {
	return executeTemplate(ctx, data, wrapperTemplate)
}

func generateGoFrRequestWrapper(ctx *gofr.Context, data *WrapperData) string {
	return executeTemplate(ctx, data, messageTemplate)
}

func generateGoFrServerHealthWrapper(ctx *gofr.Context, data *WrapperData) string {
	return executeTemplate(ctx, data, healthServerTemplate)
}

func generateGoFrClientHealth(ctx *gofr.Context, data *WrapperData) string {
	return executeTemplate(ctx, data, clientHealthTemplate)
}

func generateGoFrServer(ctx *gofr.Context, data *WrapperData) string {
	return executeTemplate(ctx, data, serverTemplate)
}

func generateGoFrClient(ctx *gofr.Context, data *WrapperData) string {
	return executeTemplate(ctx, data, clientTemplate)
}

// getPackageAndProject extracts the package name and project path from the proto definition.
func getPackageAndProject(ctx *gofr.Context, definition *proto.Proto, protoPath string) (projectPath, packageName string) {
	proto.Walk(definition,
		proto.WithOption(func(opt *proto.Option) {
			if opt.Name == "go_package" {
				packageName = path.Base(opt.Constant.Source)
			}
		}),
	)

	projectPath = path.Dir(protoPath)
	ctx.Logger.Debugf("Extracted package name: %s, project path: %s", packageName, projectPath)

	return projectPath, packageName
}

// getServices extracts services from the proto definition.
func getServices(ctx *gofr.Context, definition *proto.Proto) []ProtoService {
	var services []ProtoService

	proto.Walk(definition,
		proto.WithService(func(s *proto.Service) {
			service := ProtoService{Name: s.Name}

			for _, element := range s.Elements {
				if rpc, ok := element.(*proto.RPC); ok {
					service.Methods = append(service.Methods, ServiceMethod{
						Name:            rpc.Name,
						Request:         rpc.RequestType,
						Response:        rpc.ReturnsType,
						StreamsRequest:  rpc.StreamsRequest,
						StreamsResponse: rpc.StreamsReturns,
					})
				}
			}

			services = append(services, service)
		}),
	)

	ctx.Logger.Debugf("Extracted services: %v", services)

	return services
}
