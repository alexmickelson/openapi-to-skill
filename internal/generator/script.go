package generator

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/alexmickelson/openapi-to-skill/internal/openapi"
)

func WriteScript(outDir, name string, doc *openapi.Document, sourceURL string) (string, error) {
	hasBearer := HasBearerAuth(doc)
	commands := ExtractCommands(doc, hasBearer)
	baseURL, pathPrefix := resolvedBaseAndPrefix(doc, sourceURL)
	scriptPath := filepath.Join(outDir, "scripts", name)
	if err := os.WriteFile(scriptPath, []byte(renderScript(name, commands, hasBearer, baseURL, pathPrefix)), 0o755); err != nil {
		return "", err
	}
	return scriptPath, nil
}

// resolvedBaseAndPrefix returns the default BASE_URL (scheme+host, or servers[0].URL) and the
// path prefix extracted from the spec URL (e.g. "/api/trpc" from "/api/trpc/openapi.json").
// When servers[0].URL is present the prefix is empty because it is already encoded in the base URL.
func resolvedBaseAndPrefix(doc *openapi.Document, sourceURL string) (baseURL, pathPrefix string) {
	if len(doc.Servers) > 0 && doc.Servers[0].URL != "" {
		return doc.Servers[0].URL, ""
	}
	parsed, err := url.Parse(sourceURL)
	if err != nil || parsed.Host == "" {
		return sourceURL, ""
	}
	base := parsed.Scheme + "://" + parsed.Host
	prefix := strings.TrimSuffix(path.Dir(parsed.Path), "/")
	if prefix == "." {
		prefix = ""
	}
	return base, prefix
}

func renderScript(name string, commands []Command, hasBearer bool, defaultBaseURL, pathPrefix string) string {
	envPrefix := envVarPrefix(name)
	requiresJq := anyCommandHasBodyParams(commands)
	var builder strings.Builder
	writeGlobalVarBlock(&builder, envPrefix, hasBearer, requiresJq, defaultBaseURL)
	writeArgParseBlock(&builder, name)
	writeCommandDispatcher(&builder, commands, pathPrefix)
	return builder.String()
}

func writeGlobalVarBlock(builder *strings.Builder, envPrefix string, hasBearer, requiresJq bool, defaultBaseURL string) {
	writeLine := lineWriter(builder)
	writeLine("#!/usr/bin/env bash")
	writeLine("set -euo pipefail")
	writeLine("")
	writeLine(fmt.Sprintf(`BASE_URL="${%s_BASE_URL:-%s}"`, envPrefix, defaultBaseURL))
	if hasBearer {
		writeLine(fmt.Sprintf(`TOKEN="${%s_TOKEN:-}"`, envPrefix))
	}
	if requiresJq {
		writeLine("")
		writeLine(`command -v jq >/dev/null 2>&1 || { echo "jq is required" >&2; exit 1; }`)
	}
}

func writeArgParseBlock(builder *strings.Builder, scriptName string) {
	writeLine := lineWriter(builder)
	writeLine("")
	writeLine(`if [ $# -lt 2 ]; then`)
	writeLine(fmt.Sprintf(`  echo "usage: %s <group> <subcommand> [flags]" >&2`, scriptName))
	writeLine(`  exit 1`)
	writeLine(`fi`)
	writeLine("")
	writeLine(`_group="$1"`)
	writeLine(`_sub="$2"`)
	writeLine(`shift 2`)
	writeLine("")
}

func writeCommandDispatcher(builder *strings.Builder, commands []Command, pathPrefix string) {
	writeLine := lineWriter(builder)
	writeLine(`case "${_group} ${_sub}" in`)
	for _, command := range commands {
		writeCaseBlock(builder, command, pathPrefix)
	}
	writeLine(`  *)`)
	writeLine(`    echo "unknown command: ${_group} ${_sub}" >&2`)
	writeLine(`    exit 1`)
	writeLine(`    ;;`)
	writeLine(`esac`)
}

func writeCaseBlock(builder *strings.Builder, command Command, pathPrefix string) {
	writeLine := lineWriter(builder)
	writeLine(fmt.Sprintf(`  "%s %s")`, command.Group, command.Sub))
	writeFlagParsing(builder, allParamsForCommand(command))
	writeURLConstruction(builder, command, pathPrefix)
	writeQueryStringAppend(builder, command.QueryParams)
	writeBodyConstruction(builder, command.BodyParams)
	writeCurlInvocation(builder, command)
	writeLine(`    ;;`)
}

func writeFlagParsing(builder *strings.Builder, params []Param) {
	if len(params) == 0 {
		return
	}
	writeLine := lineWriter(builder)
	for _, param := range params {
		writeLine(fmt.Sprintf(`    %s=""`, param.BashVar))
	}
	writeLine(`    while [ $# -gt 0 ]; do`)
	writeLine(`      case "$1" in`)
	for _, param := range params {
		writeLine(fmt.Sprintf(`        %s) %s="$2"; shift 2 ;;`, param.Flag, param.BashVar))
	}
	writeLine(`        *) echo "unknown flag: $1" >&2; exit 1 ;;`)
	writeLine(`      esac`)
	writeLine(`    done`)
}

func writeURLConstruction(builder *strings.Builder, command Command, pathPrefix string) {
	urlExpression := "${BASE_URL}" + pathPrefix + command.Path
	for _, param := range command.PathParams {
		urlExpression = strings.ReplaceAll(urlExpression, "{"+param.Name+"}", "${"+param.BashVar+"}")
	}
	lineWriter(builder)(fmt.Sprintf(`    _url="%s"`, urlExpression))
}

func writeQueryStringAppend(builder *strings.Builder, queryParams []Param) {
	if len(queryParams) == 0 {
		return
	}
	writeLine := lineWriter(builder)
	writeLine(`    _qs=""`)
	for _, param := range queryParams {
		writeLine(fmt.Sprintf(`    [ -n "$%s" ] && _qs="${_qs}&%s=${%s}"`, param.BashVar, param.Name, param.BashVar))
	}
	writeLine(`    [ -n "$_qs" ] && _url="${_url}?${_qs#&}"`)
}

func writeBodyConstruction(builder *strings.Builder, bodyParams []Param) {
	if len(bodyParams) == 0 {
		return
	}
	lineWriter(builder)(renderJqBodyStatement(bodyParams))
}

func writeCurlInvocation(builder *strings.Builder, command Command) {
	curlArgs := []string{fmt.Sprintf(`    curl -sf -X %s "$_url"`, command.Method)}
	if len(command.BodyParams) > 0 {
		curlArgs = append(curlArgs, `      -H "Content-Type: application/json"`)
	}
	if command.HasBearer {
		curlArgs = append(curlArgs, `      -H "Authorization: Bearer ${TOKEN}"`)
	}
	if len(command.BodyParams) > 0 {
		curlArgs = append(curlArgs, `      -d "$_body"`)
	}
	lineWriter(builder)(strings.Join(curlArgs, " \\\n"))
}

func renderJqBodyStatement(params []Param) string {
	var jqArgs, jsonFields []string
	for _, param := range params {
		jqVarName := param.BashVar[1:] // strip leading underscore to get a plain jq variable name
		switch param.Type {
		case "integer", "number", "boolean":
			jqArgs = append(jqArgs, fmt.Sprintf(`      --argjson %s "$%s"`, jqVarName, param.BashVar))
		default:
			jqArgs = append(jqArgs, fmt.Sprintf(`      --arg %s "$%s"`, jqVarName, param.BashVar))
		}
		jsonFields = append(jsonFields, fmt.Sprintf(`"%s":$%s`, param.Name, jqVarName))
	}
	return fmt.Sprintf("    _body=$(jq -n \\\n%s \\\n      '{%s}')",
		strings.Join(jqArgs, " \\\n"),
		strings.Join(jsonFields, ","))
}

func allParamsForCommand(command Command) []Param {
	params := make([]Param, 0, len(command.PathParams)+len(command.QueryParams)+len(command.BodyParams))
	params = append(params, command.PathParams...)
	params = append(params, command.QueryParams...)
	params = append(params, command.BodyParams...)
	return params
}

func anyCommandHasBodyParams(commands []Command) bool {
	for _, command := range commands {
		if len(command.BodyParams) > 0 {
			return true
		}
	}
	return false
}

func envVarPrefix(name string) string {
	return strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
}

func lineWriter(builder *strings.Builder) func(string) {
	return func(line string) {
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
}

