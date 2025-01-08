package wrap

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"text/template"

	"github.com/emicklei/proto"
	"gofr.dev/pkg/gofr"
)

const filePerm = 0644

var (
	ErrNoProtoFile              = errors.New("proto file path is required")
	ErrOpeningProtoFile         = errors.New("error opening the proto file")
	ErrFailedToParseProto       = errors.New("failed to parse proto file")
	ErrGeneratingWrapper        = errors.New("error generating the wrapper code from the proto file")
	ErrWritingWrapperFile       = errors.New("error writing the generated wrapper to the file")
	ErrGeneratingServerTemplate = errors.New("error generating the gRPC server file template")
	ErrWritingServerTemplate    = errors.New("error writing the generated server template to the file")
)

// ServiceMethod represents a method in a proto service.
type ServiceMethod struct {
	Name      string
	Request   string
	Response  string
	Streaming bool
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
}

func GenerateWrapper(ctx *gofr.Context) (any, error) {
	protoPath := ctx.Param("proto")
	if protoPath == "" {
		return nil, ErrNoProtoFile
	}

	file, err := os.Open(protoPath)
	if err != nil {
		ctx.Errorf("Failed to open proto file: %v", err)

		return nil, ErrOpeningProtoFile
	}

	defer file.Close()

	parser := proto.NewParser(file)

	definition, err := parser.Parse()
	if err != nil {
		ctx.Errorf("Failed to parse proto file: %v", err)

		return nil, ErrFailedToParseProto
	}

	var (
		// Extracting package and project path from go_package option.
		projectPath, packageName = getPackageAndProject(definition)
		// Extract the services.
		services = getServices(definition)
	)

	for _, service := range services {
		wrapperData := WrapperData{
			Package:  packageName,
			Service:  service.Name,
			Methods:  service.Methods,
			Requests: uniqueRequestTypes(service.Methods),
		}

		generatedCode := generateWrapperCode(ctx, &wrapperData)
		if generatedCode == "" {
			return nil, ErrGeneratingWrapper
		}

		outputFilePath := path.Join(projectPath, fmt.Sprintf("%s.gofr.go", strings.ToLower(service.Name)))

		err := os.WriteFile(outputFilePath, []byte(generatedCode), filePerm)
		if err != nil {
			ctx.Errorf("Failed to write file %s: %v", outputFilePath, err)

			return nil, ErrWritingWrapperFile
		}

		fmt.Printf("Generated wrapper for service %s at %s\n", service.Name, outputFilePath)

		generatedgRPCCode := generategRPCCode(ctx, &wrapperData)
		if generatedgRPCCode == "" {
			return nil, ErrGeneratingServerTemplate
		}

		outputFilePath = path.Join(projectPath, fmt.Sprintf("%sServer.go", strings.ToLower(service.Name)))

		err = os.WriteFile(outputFilePath, []byte(generatedgRPCCode), filePerm)
		if err != nil {
			ctx.Errorf("Failed to write file %s: %v", outputFilePath, err)

			return nil, ErrWritingServerTemplate
		}

		fmt.Printf("Generated server template for service %s at %s\n", service.Name, outputFilePath)
	}

	return "Successfully generated all wrappers for gRPC services", nil
}

// Extract unique request types from methods.
func uniqueRequestTypes(methods []ServiceMethod) []string {
	requests := make(map[string]bool)
	req := make([]string, 0)

	for _, method := range methods {
		if !method.Streaming {
			requests[method.Request] = true
		}
	}

	for method := range requests {
		req = append(req, method)
	}

	return req
}

// Generate wrapper code using the template.
func generateWrapperCode(ctx *gofr.Context, data *WrapperData) string {
	var buf bytes.Buffer

	tmplInstance := template.Must(template.New("wrapper").Parse(wrapperTemplate))

	err := tmplInstance.Execute(&buf, data)
	if err != nil {
		ctx.Errorf("Template execution failed: %v", err)

		return ""
	}

	return buf.String()
}

// Generate wrapper code using the template.
func generategRPCCode(ctx *gofr.Context, data *WrapperData) string {
	var buf bytes.Buffer

	tmplInstance := template.Must(template.New("wrapper").Parse(serverTemplate))

	err := tmplInstance.Execute(&buf, data)
	if err != nil {
		ctx.Errorf("Template execution failed: %v", err)
		return ""
	}

	return buf.String()
}

func getPackageAndProject(definition *proto.Proto) (projectPath, packageName string) {
	proto.Walk(definition,
		proto.WithOption(func(opt *proto.Option) {
			if opt.Name == "go_package" {
				projectPath = opt.Constant.Source
				packageName = path.Base(opt.Constant.Source)
			}
		}),
	)

	return projectPath, packageName
}

func getServices(definition *proto.Proto) []ProtoService {
	var services []ProtoService

	proto.Walk(definition,
		proto.WithService(func(s *proto.Service) {
			service := ProtoService{Name: s.Name}

			for _, element := range s.Elements {
				if rpc, ok := element.(*proto.RPC); ok {
					method := ServiceMethod{
						Name:      rpc.Name,
						Request:   rpc.RequestType,
						Response:  rpc.ReturnsType,
						Streaming: rpc.StreamsReturns || rpc.StreamsRequest,
					}
					service.Methods = append(service.Methods, method)
				}
			}

			services = append(services, service)
		}),
	)

	return services
}
