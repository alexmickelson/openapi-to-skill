package generator

import (
	"encoding/json"
	"sort"
	"strings"
	"unicode"

	"github.com/alexmickelson/openapi-to-skill/internal/openapi"
)

type Param struct {
	Name       string // spec name,  e.g. "user-id"
	BashVar    string // bash-safe variable, e.g. "_user_id"
	Flag       string // CLI flag, e.g. "--user-id"
	Type       string // "string" | "integer" | "number" | "boolean"
	Required   bool
	SchemaRef  string // schema filename (without .json) to reference in help; set from $ref or synthesized
	SchemaJSON string // JSON to write to schema/<SchemaRef>.json for inline object params
}

type Command struct {
	Group       string
	Sub         string
	Method      string
	Path        string
	Summary     string // operation summary or description
	PathParams  []Param
	QueryParams []Param
	BodyParams  []Param
	SchemaName  string // requestBody $ref component name; empty when inline or absent
	HasBearer   bool
}

func HasBearerAuth(doc *openapi.Document) bool {
	for _, scheme := range doc.Components.SecuritySchemes {
		if scheme != nil &&
			strings.EqualFold(scheme.Type, "http") &&
			strings.EqualFold(scheme.Scheme, "bearer") {
			return true
		}
	}
	return false
}

func ExtractCommands(doc *openapi.Document, hasBearer bool) []Command {
	operations := sortedOperations(doc.Paths)
	commands := make([]Command, 0, len(operations))
	for _, operation := range operations {
		commands = append(commands, operationToCommand(doc, operation, hasBearer))
	}
	return commands
}

type pathOperation struct {
	path   string
	method string
	op     *openapi.Operation
}

func sortedOperations(paths map[string]openapi.PathItem) []pathOperation {
	var operations []pathOperation
	for path, item := range paths {
		for method, op := range operationsInPathItem(&item) {
			operations = append(operations, pathOperation{path, method, op})
		}
	}
	sort.Slice(operations, func(i, j int) bool {
		if operations[i].path != operations[j].path {
			return operations[i].path < operations[j].path
		}
		return operations[i].method < operations[j].method
	})
	return operations
}

func operationToCommand(doc *openapi.Document, operation pathOperation, hasBearer bool) Command {
	summary := operation.op.Summary
	if summary == "" {
		summary = operation.op.Description
	}
	command := Command{
		Group:     commandGroup(operation.op, operation.path),
		Sub:       commandVerb(operation.method, operation.path, operation.op),
		Method:    strings.ToUpper(operation.method),
		Path:      operation.path,
		Summary:   summary,
		HasBearer: hasBearer,
	}
	command.PathParams, command.QueryParams = extractURLParams(operation.op.Parameters)
	command.BodyParams, command.SchemaName = extractBodyParams(doc, operation.op.RequestBody)
	return command
}

func extractURLParams(parameters []openapi.Parameter) (pathParams, queryParams []Param) {
	for _, parameter := range parameters {
		param := paramFromSpec(parameter.Name, parameter.Schema)
		param.Required = parameter.Required || parameter.In == "path"
		switch parameter.In {
		case "path":
			pathParams = append(pathParams, param)
		case "query":
			queryParams = append(queryParams, param)
		}
	}
	return pathParams, queryParams
}

func extractBodyParams(doc *openapi.Document, requestBody *openapi.RequestBody) (params []Param, schemaName string) {
	if requestBody == nil {
		return nil, ""
	}
	mediaType, ok := requestBody.Content["application/json"]
	if !ok || mediaType.Schema == nil {
		return nil, ""
	}
	schema := mediaType.Schema
	if schema.Ref != "" {
		schemaName = schemaRefName(schema.Ref)
		schema = resolveSchemaRef(doc, schemaName, schema)
	}
	for propertyName, propertySchema := range schema.Properties {
		param := paramFromSpec(propertyName, propertySchema)
		param.Required = contains(schema.Required, propertyName)
		if propertySchema != nil && propertySchema.Ref != "" {
			param.SchemaRef = schemaRefName(propertySchema.Ref)
		} else if propertySchema != nil && (propertySchema.Type == "object" || len(propertySchema.Properties) > 0) {
			if data, err := json.MarshalIndent(propertySchema, "", "  "); err == nil {
				param.SchemaRef = propertyName
				param.SchemaJSON = string(data)
			}
		}
		params = append(params, param)
	}
	sort.Slice(params, func(i, j int) bool {
		return params[i].Name < params[j].Name
	})
	return params, schemaName
}

func resolveSchemaRef(doc *openapi.Document, name string, fallback *openapi.Schema) *openapi.Schema {
	if doc.Components.Schemas != nil {
		if resolved, ok := doc.Components.Schemas[name]; ok {
			return resolved
		}
	}
	return fallback
}

func operationsInPathItem(item *openapi.PathItem) map[string]*openapi.Operation {
	operations := make(map[string]*openapi.Operation)
	if item.Get != nil {
		operations["get"] = item.Get
	}
	if item.Post != nil {
		operations["post"] = item.Post
	}
	if item.Put != nil {
		operations["put"] = item.Put
	}
	if item.Patch != nil {
		operations["patch"] = item.Patch
	}
	if item.Delete != nil {
		operations["delete"] = item.Delete
	}
	return operations
}

func commandGroup(op *openapi.Operation, path string) string {
	if len(op.Tags) > 0 && op.Tags[0] != "" {
		return strings.ToLower(op.Tags[0])
	}
	return firstStaticPathSegment(path)
}

func firstStaticPathSegment(path string) string {
	for _, segment := range strings.Split(strings.Trim(path, "/"), "/") {
		if segment != "" && !strings.HasPrefix(segment, "{") {
			return strings.ToLower(segment)
		}
	}
	return "api"
}

func commandVerb(method, path string, op *openapi.Operation) string {
	if op.OperationID != "" {
		return leadingVerbFromOperationID(op.OperationID)
	}
	return verbFromHTTPMethod(method, path)
}

func leadingVerbFromOperationID(operationID string) string {
	// Strip dot-namespace prefix (e.g. "settings." from "settings.allCoursesSettings")
	if dotIdx := strings.LastIndex(operationID, "."); dotIdx >= 0 {
		operationID = operationID[dotIdx+1:]
	}
	kebabID := camelToKebab(operationID)
	if hyphenIndex := strings.Index(kebabID, "-"); hyphenIndex > 0 {
		return kebabID[:hyphenIndex]
	}
	return kebabID
}

func verbFromHTTPMethod(method, path string) string {
	pathHasParam := strings.Contains(path, "{")
	switch strings.ToUpper(method) {
	case "GET":
		if pathHasParam {
			return "get"
		}
		return "list"
	case "POST":
		return "create"
	case "PUT", "PATCH":
		return "update"
	case "DELETE":
		return "delete"
	}
	return strings.ToLower(method)
}

func camelToKebab(camel string) string {
	var builder strings.Builder
	for index, r := range camel {
		if index > 0 && unicode.IsUpper(r) {
			builder.WriteRune('-')
		}
		builder.WriteRune(unicode.ToLower(r))
	}
	return builder.String()
}

func paramFromSpec(name string, schema *openapi.Schema) Param {
	schemaType := "string"
	if schema != nil && schema.Type != "" {
		schemaType = schema.Type
	}
	return Param{
		Name:    name,
		BashVar: toBashVar(name),
		Flag:    "--" + name,
		Type:    schemaType,
	}
}

func toBashVar(name string) string {
	var builder strings.Builder
	builder.WriteRune('_')
	for _, r := range name {
		if r == '-' || r == '.' {
			builder.WriteRune('_')
		} else {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func schemaRefName(ref string) string {
	parts := strings.Split(ref, "/")
	return parts[len(parts)-1]
}

func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}
