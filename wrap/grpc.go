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

// googleProtobufType returns the Go package type for a given Google Protobuf type.
//
//nolint:gocyclo,funlen // This function uses a large switch statement for type mapping, which is expected.
func googleProtobufType(tpe string) (string, bool) {
	switch tpe {
	case "google.protobuf.Any":
		return "anypb.Any", true
	case "google.protobuf.Api":
		return "apipb.Api", true
	case "google.protobuf.Method":
		return "apipb.Method", true
	case "google.protobuf.Mixin":
		return "apipb.Mixin", true
	case "google.protobuf.FileDescriptorSet":
		return "descriptorpb.FileDescriptorSet", true
	case "google.protobuf.FileDescriptorProto":
		return "descriptorpb.FileDescriptorProto", true
	case "google.protobuf.DescriptorProto":
		return "descriptorpb.DescriptorProto", true
	case "google.protobuf.ExtensionRangeOptions":
		return "descriptorpb.ExtensionRangeOptions", true
	case "google.protobuf.FieldDescriptorProto":
		return "descriptorpb.FieldDescriptorProto", true
	case "google.protobuf.OneofDescriptorProto":
		return "descriptorpb.OneofDescriptorProto", true
	case "google.protobuf.EnumDescriptorProto":
		return "descriptorpb.EnumDescriptorProto", true
	case "google.protobuf.EnumValueDescriptorProto":
		return "descriptorpb.EnumValueDescriptorProto", true
	case "google.protobuf.ServiceDescriptorProto":
		return "descriptorpb.ServiceDescriptorProto", true
	case "google.protobuf.MethodDescriptorProto":
		return "descriptorpb.MethodDescriptorProto", true
	case "google.protobuf.FileOptions":
		return "descriptorpb.FileOptions", true
	case "google.protobuf.MessageOptions":
		return "descriptorpb.MessageOptions", true
	case "google.protobuf.FieldOptions":
		return "descriptorpb.FieldOptions", true
	case "google.protobuf.OneofOptions":
		return "descriptorpb.OneofOptions", true
	case "google.protobuf.EnumOptions":
		return "descriptorpb.EnumOptions", true
	case "google.protobuf.EnumValueOptions":
		return "descriptorpb.EnumValueOptions", true
	case "google.protobuf.ServiceOptions":
		return "descriptorpb.ServiceOptions", true
	case "google.protobuf.MethodOptions":
		return "descriptorpb.MethodOptions", true
	case "google.protobuf.UninterpretedOption":
		return "descriptorpb.UninterpretedOption", true
	case "google.protobuf.FeatureSet":
		return "descriptorpb.FeatureSet", true
	case "google.protobuf.FeatureSetDefaults":
		return "descriptorpb.FeatureSetDefaults", true
	case "google.protobuf.SourceCodeInfo":
		return "descriptorpb.SourceCodeInfo", true
	case "google.protobuf.GeneratedCodeInfo":
		return "descriptorpb.GeneratedCodeInfo", true
	case "google.protobuf.SymbolVisibility":
		return "descriptorpb.SymbolVisibility", true
	case "google.protobuf.Duration":
		return "durationpb.Duration", true
	case "google.protobuf.Empty":
		return "emptypb.Empty", true
	case "google.protobuf.FieldMask":
		return "fieldmaskpb.FieldMask", true
	case "google.protobuf.GoFeatures":
		return "gofeaturespb.GoFeatures", true
	case "google.protobuf.SourceContext":
		return "sourcecontextpb.SourceContext", true
	case "google.protobuf.Struct":
		return "structpb.Struct", true
	case "google.protobuf.Value":
		return "structpb.Value", true
	case "google.protobuf.NullValue":
		return "structpb.NullValue", true
	case "google.protobuf.ListValue":
		return "structpb.ListValue", true
	case "google.protobuf.Timestamp":
		return "timestamppb.Timestamp", true
	case "google.protobuf.Type":
		return "typepb.Type", true
	case "google.protobuf.Field":
		return "typepb.Field", true
	case "google.protobuf.Enum":
		return "typepb.Enum", true
	case "google.protobuf.EnumValue":
		return "typepb.EnumValue", true
	case "google.protobuf.Option":
		return "typepb.Option", true
	case "google.protobuf.Syntax":
		return "typepb.Syntax", true
	case "google.protobuf.DoubleValue":
		return "wrapperspb.DoubleValue", true
	case "google.protobuf.FloatValue":
		return "wrapperspb.FloatValue", true
	case "google.protobuf.Int64Value":
		return "wrapperspb.Int64Value", true
	case "google.protobuf.UInt64Value":
		return "wrapperspb.UInt64Value", true
	case "google.protobuf.Int32Value":
		return "wrapperspb.Int32Value", true
	case "google.protobuf.UInt32Value":
		return "wrapperspb.UInt32Value", true
	case "google.protobuf.BoolValue":
		return "wrapperspb.BoolValue", true
	case "google.protobuf.StringValue":
		return "wrapperspb.StringValue", true
	case "google.protobuf.BytesValue":
		return "wrapperspb.BytesValue", true
	default:
		return "", false
	}
}

// ServiceMethod represents a method in a proto service.
type ServiceMethod struct {
	Name            string
	Request         string
	Response        string
	RawRequest      string
	RawResponse     string
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
	Requests []ServiceRequest
	Source   string
	Imports  []string
}

type ServiceRequest struct {
	Request    string
	RawRequest string
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
		ctx.Error(ErrNoProtoFile)
		return nil, ErrNoProtoFile
	}

	definition, err := parseProtoFile(ctx, protoPath)
	if err != nil {
		ctx.Errorf("Failed to parse proto file: %v", err)
		return nil, err
	}

	imports := getImports(ctx, definition, protoPath)
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
			Imports:  imports,
		}

		if err := generateFiles(ctx, projectPath, service.Name, &wrapperData, requests, options...); err != nil {
			return nil, err
		}
	}

	ctx.Info("Successfully generated all files for GoFr integrated gRPC servers/clients")

	return "Successfully generated all files for GoFr integrated gRPC servers/clients", nil
}

// parseProtoFile opens and parses the proto file.
func parseProtoFile(ctx *gofr.Context, protoPath string) (*proto.Proto, error) {
	file, err := os.Open(protoPath)
	if err != nil {
		ctx.Errorf("Failed to open proto file: %v", err)
		return nil, ErrOpeningProtoFile
	}

	defer func() { _ = file.Close() }()

	parser := proto.NewParser(file)

	definition, err := parser.Parse()
	if err != nil {
		ctx.Errorf("Failed to parse proto file: %v", err)
		return nil, ErrFailedToParseProto
	}

	return definition, nil
}

// generateFiles generates files for a given service.
func generateFiles(ctx *gofr.Context, projectPath, serviceName string, wrapperData *WrapperData,
	requests []ServiceRequest, options ...FileType) error {
	for _, option := range options {
		if option.FileSuffix == serverRequestFile {
			wrapperData.Requests = requests
		}

		generatedCode := option.CodeGenerator(ctx, wrapperData)
		if generatedCode == "" {
			ctx.Errorf("Failed to generate code for service %s with file suffix %s", serviceName, option.FileSuffix)
			return ErrGeneratingWrapper
		}

		outputFilePath := getOutputFilePath(projectPath, serviceName, option.FileSuffix)
		if err := os.WriteFile(outputFilePath, []byte(generatedCode), filePerm); err != nil {
			ctx.Errorf("Failed to write file %s: %v", outputFilePath, err)
			return ErrWritingFile
		}

		ctx.Infof("Generated file for service %s at %s", serviceName, outputFilePath)
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
func getRequests(ctx *gofr.Context, services []ProtoService) []ServiceRequest {
	requests := make(map[string]ServiceRequest)

	for _, service := range services {
		for _, method := range service.Methods {
			requests[method.Request] = ServiceRequest{
				Request:    method.Request,
				RawRequest: method.RawRequest,
			}
		}
	}

	ctx.Debugf("Extracted unique request types: %v", requests)

	return mapValuesToSlice(requests)
}

// uniqueRequestTypes extracts unique request types from methods.
func uniqueRequestTypes(ctx *gofr.Context, methods []ServiceMethod) []ServiceRequest {
	requests := make(map[string]ServiceRequest)

	for _, method := range methods {
		requests[method.Request] = ServiceRequest{
			Request:    method.Request,
			RawRequest: method.RawRequest,
		} // Include all request types
	}

	ctx.Debugf("Extracted unique request types for methods: %v", requests)

	return mapValuesToSlice(requests)
}

// mapValuesToSlice converts a map's keys to a slice.
func mapValuesToSlice(m map[string]ServiceRequest) []ServiceRequest {
	values := make([]ServiceRequest, 0, len(m))
	for _, value := range m {
		values = append(values, value)
	}

	return values
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
		ctx.Errorf("Template execution failed: %v", err)
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
	const goPackage = "go_package"

	proto.Walk(definition,
		proto.WithOption(func(opt *proto.Option) {
			if opt.Name == goPackage {
				packageName = path.Base(opt.Constant.Source)
			}
		}),
	)

	projectPath = path.Dir(protoPath)
	ctx.Debugf("Extracted package name: %s, project path: %s", packageName, projectPath)

	return projectPath, packageName
}

// getImports extracts the import directories from google protobufs and relative go_package proto definitions.
func getImports(ctx *gofr.Context, definition *proto.Proto, protoPath string) []string {
	const goPackage = "go_package"

	imports := []string{}
	googleImports := map[string]string{
		"google/protobuf/any.proto":            "anypb \"google.golang.org/protobuf/types/known/anypb\"",
		"google/protobuf/api.proto":            "apipb \"google.golang.org/protobuf/types/known/apipb\"",
		"google/protobuf/descriptor.proto":     "descriptorpb \"google.golang.org/protobuf/types/descriptorpb\"",
		"google/protobuf/duration.proto":       "durationpb \"google.golang.org/protobuf/types/known/durationpb\"",
		"google/protobuf/empty.proto":          "emptypb \"google.golang.org/protobuf/types/known/emptypb\"",
		"google/protobuf/field_mask.proto":     "fieldmaskpb \"google.golang.org/protobuf/types/known/fieldmaskpb\"",
		"google/protobuf/go_features.proto":    "gofeaturespb \"google.golang.org/protobuf/types/gofeaturespb\"",
		"google/protobuf/source_context.proto": "sourcecontextpb \"google.golang.org/protobuf/types/known/sourcecontextpb\"",
		"google/protobuf/struct.proto":         "structpb \"google.golang.org/protobuf/types/known/structpb\"",
		"google/protobuf/timestamp.proto":      "timestamppb \"google.golang.org/protobuf/types/known/timestamppb\"",
		"google/protobuf/type.proto":           "typepb \"google.golang.org/protobuf/types/known/typepb\"",
		"google/protobuf/wrappers.proto":       "wrapperspb \"google.golang.org/protobuf/types/known/wrapperspb\"",
	}

	for _, elem := range definition.Elements {
		imported, ok := elem.(*proto.Import)
		if !ok {
			continue
		}

		if googleImport, ok := googleImports[imported.Filename]; ok {
			imports = append(imports, googleImport)
		} else {
			lastIndex := strings.LastIndex(protoPath, "/")

			newProto, err := parseProtoFile(ctx, protoPath[:lastIndex+1]+imported.Filename)
			if err != nil {
				ctx.Errorf("Failed to parse imported proto file %s: %v", imported.Filename, err)
				continue
			}

			packageSource := ""
			packageName := ""

			for _, newElem := range newProto.Elements {
				if pkg, ok := newElem.(*proto.Option); ok && pkg.Name == goPackage {
					packageSource = pkg.Constant.Source
				}

				if pkg, ok := newElem.(*proto.Package); ok {
					packageName = pkg.Name
				}
			}

			lastPiece := strings.LastIndex(packageName, ".")
			formattedImport := fmt.Sprintf("%s %q", packageName[lastPiece+1:], packageSource)
			imports = append(imports, formattedImport)
		}
	}

	return imports
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
						Request:         getProperType(rpc.RequestType),
						Response:        getProperType(rpc.ReturnsType),
						RawRequest:      getRawType(rpc.RequestType),
						RawResponse:     getRawType(rpc.ReturnsType),
						StreamsRequest:  rpc.StreamsRequest,
						StreamsResponse: rpc.StreamsReturns,
					})
				}
			}

			services = append(services, service)
		}),
	)

	ctx.Debugf("Extracted services: %v", services)

	return services
}

func getProperType(tpe string) string {
	if strings.HasPrefix(tpe, "google.protobuf.") {
		if protobuf, ok := googleProtobufType(tpe); ok {
			return protobuf
		}

		return tpe
	} else if strings.Contains(tpe, ".") {
		lastIndex := strings.LastIndex(tpe, ".")
		submoduleIndex := strings.LastIndex(tpe[:lastIndex], ".")

		if submoduleIndex != -1 {
			return fmt.Sprintf("%s.%s", tpe[submoduleIndex+1:lastIndex], tpe[lastIndex+1:])
		}
	}

	return tpe
}

func getRawType(tpe string) string {
	lastIndex := strings.LastIndex(tpe, ".")
	return tpe[lastIndex+1:]
}
