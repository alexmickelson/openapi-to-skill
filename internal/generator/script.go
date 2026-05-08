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

// cmdGroup groups commands by their Group name, preserving order.
type cmdGroup struct {
	name     string
	commands []Command
}

func groupCommands(commands []Command) []cmdGroup {
	seen := make(map[string]int)
	var groups []cmdGroup
	for _, cmd := range commands {
		if idx, ok := seen[cmd.Group]; ok {
			groups[idx].commands = append(groups[idx].commands, cmd)
		} else {
			seen[cmd.Group] = len(groups)
			groups = append(groups, cmdGroup{cmd.Group, []Command{cmd}})
		}
	}
	return groups
}

// resolvedBaseAndPrefix returns the default BASE_URL (scheme+host, or servers[0].URL) and the
// path prefix extracted from the spec URL (e.g. "/api/trpc" from "/api/trpc/openapi.json").
// When servers[0].URL is present the prefix is empty because it is already encoded in the base URL.
func resolvedBaseAndPrefix(doc *openapi.Document, sourceURL string) (baseURL, pathPrefix string) {
	parsed, err := url.Parse(sourceURL)
	if err != nil || parsed.Host == "" {
		return sourceURL, ""
	}
	origin := parsed.Scheme + "://" + parsed.Host

	if len(doc.Servers) > 0 && doc.Servers[0].URL != "" {
		serverURL := doc.Servers[0].URL
		// Relative server URL (e.g. "/api/trpc"): embed it as prefix, keep origin as base.
		if !strings.HasPrefix(serverURL, "http://") && !strings.HasPrefix(serverURL, "https://") {
			return origin, strings.TrimSuffix(serverURL, "/")
		}
		return serverURL, ""
	}

	prefix := strings.TrimSuffix(path.Dir(parsed.Path), "/")
	if prefix == "." {
		prefix = ""
	}
	return origin, prefix
}

func renderScript(name string, commands []Command, hasBearer bool, defaultBaseURL, pathPrefix string) string {
	envPrefix := envVarPrefix(name)
	requiresJq := anyCommandNeedsJq(commands)
	groups := groupCommands(commands)
	return strings.Join([]string{
		globalVarBlock(envPrefix, hasBearer, requiresJq, defaultBaseURL),
		helpFunctions(name, groups),
		argParseBlock(),
		commandDispatcher(commands, groups, pathPrefix),
	}, "\n\n")
}

func globalVarBlock(envPrefix string, hasBearer, requiresJq bool, defaultBaseURL string) string {
	parts := []string{fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${%s_BASE_URL:-%s}"
_script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -t 2 ]; then
  _RED=$'\033[0;31m'
  _RESET=$'\033[0m'
else
  _RED=''
  _RESET=''
fi`, envPrefix, defaultBaseURL)}
	if hasBearer {
		parts = append(parts, fmt.Sprintf(`TOKEN="${%s_TOKEN:-}"`, envPrefix))
	}
	if requiresJq {
		parts = append(parts, `command -v jq >/dev/null 2>&1 || { echo "jq is required" >&2; exit 1; }`)
	}
	return strings.Join(parts, "\n")
}

func helpFunctions(name string, groups []cmdGroup) string {
	groupLines := make([]string, len(groups))
	for i, g := range groups {
		subs := make([]string, len(g.commands))
		for j, cmd := range g.commands {
			subs[j] = cmd.Sub
		}
		groupLines[i] = fmt.Sprintf(`  echo "  %-16s %s"`, g.name, strings.Join(subs, "  "))
	}

	groupNames := make([]string, len(groups))
	caseParts := make([]string, len(groups))
	for i, g := range groups {
		groupNames[i] = g.name
		caseParts[i] = groupCaseBlock(name, g)
	}

	return fmt.Sprintf(`_show_help() {
  echo "usage: %s <group> <subcommand> [flags]"
  echo ""
  echo "Groups:"
%s
}

_show_group_help() {
  case "$1" in
%s
    *)
      echo "${_RED}unknown group: '$1'${_RESET}" >&2
      echo "Available groups: %s" >&2
      return 1
      ;;
  esac
}`,

		name,
		strings.Join(groupLines, "\n"),
		strings.Join(caseParts, "\n"),
		strings.Join(groupNames, "  "))
}

func groupCaseBlock(scriptName string, g cmdGroup) string {
	subLines := make([]string, 0, len(g.commands)*3)
	for _, cmd := range g.commands {
		flagSummary := paramFlagSummary(allParamsForCommand(cmd))
		if flagSummary != "" {
			subLines = append(subLines, fmt.Sprintf(`      echo "  %-12s %s"`, cmd.Sub, flagSummary))
		} else {
			subLines = append(subLines, fmt.Sprintf(`      echo "  %s"`, cmd.Sub))
		}
		if cmd.Summary != "" {
			subLines = append(subLines, fmt.Sprintf(`      echo "               %s"`, cmd.Summary))
		}
		for _, p := range cmd.BodyParams {
			if p.SchemaRef != "" {
				subLines = append(subLines,
					fmt.Sprintf(`      echo "               --%s schema: schema/%s.json"`, p.Name, p.SchemaRef),
				)
			}
		}
		if cmd.SchemaName != "" {
			subLines = append(subLines,
				fmt.Sprintf(`      echo "               request body schema: schema/%s.json"`, cmd.SchemaName),
			)
		}
	}
	return fmt.Sprintf(`    %s)
      echo "usage: %s %s <subcommand> [flags]"
      echo ""
      echo "Subcommands:"
%s
      ;;`,
		g.name, scriptName, g.name, strings.Join(subLines, "\n"))
}

// paramFlagSummary returns a one-line summary of flags, e.g. "--id <string>* --name <string>"
// where * marks required params.
func paramFlagSummary(params []Param) string {
	if len(params) == 0 {
		return ""
	}
	parts := make([]string, len(params))
	for i, p := range params {
		s := fmt.Sprintf("--%s <%s>", p.Name, p.Type)
		if p.Required {
			s += "*"
		}
		parts[i] = s
	}
	return strings.Join(parts, "  ")
}

func argParseBlock() string {
	return `if [ $# -eq 0 ]; then
  _show_help
  exit 0
fi

_group="$1"
shift

if [ $# -eq 0 ] || [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ]; then
  _show_group_help "$_group" || exit 1
  exit 0
fi

_sub="$1"
shift`
}

func commandDispatcher(commands []Command, groups []cmdGroup, pathPrefix string) string {
	caseParts := make([]string, len(commands))
	for i, cmd := range commands {
		caseParts[i] = caseBlock(cmd, pathPrefix)
	}
	groupNames := make([]string, len(groups))
	for i, g := range groups {
		groupNames[i] = g.name
	}
	return fmt.Sprintf(`case "${_group} ${_sub}" in
%s
  *)
    case "$_group" in
      %s)
        echo "${_RED}unknown subcommand '$_sub' for group '$_group'${_RESET}" >&2
        _show_group_help "$_group" >&2
        ;;
      *)
        echo "${_RED}unknown group: '$_group'${_RESET}" >&2
        _show_help >&2
        ;;
    esac
    exit 1
    ;;
esac`,
		strings.Join(caseParts, "\n"),
		strings.Join(groupNames, "|"))
}

func caseBlock(command Command, pathPrefix string) string {
	parts := []string{
		fmt.Sprintf(`  "%s %s")`, command.Group, command.Sub),
		flagParsing(command, pathPrefix),
		urlConstruction(command, pathPrefix),
	}
	if qs := queryStringAppend(command.QueryParams); qs != "" {
		parts = append(parts, qs)
	}
	if body := bodyConstruction(command.BodyParams); body != "" {
		parts = append(parts, body)
	}
	parts = append(parts, curlInvocation(command), "    ;;")
	return strings.Join(parts, "\n")
}

func flagParsing(command Command, pathPrefix string) string {
	params := allParamsForCommand(command)

	initLines := make([]string, len(params))
	for i, p := range params {
		initLines[i] = fmt.Sprintf(`    %s=""`, p.BashVar)
	}
	init := ""
	if len(initLines) > 0 {
		init = strings.Join(initLines, "\n") + "\n"
	}

	flagCases := make([]string, len(params))
	for i, p := range params {
		flagCases[i] = fmt.Sprintf(`        %s) %s="$2"; shift 2 ;;`, p.Flag, p.BashVar)
	}

	var summaryLine string
	if command.Summary != "" {
		summaryLine = fmt.Sprintf(`          echo "%s %s - %s"`, command.Group, command.Sub, command.Summary)
	} else {
		summaryLine = fmt.Sprintf(`          echo "%s %s"`, command.Group, command.Sub)
	}
	helpLines := []string{
		summaryLine,
		fmt.Sprintf(`          echo "  %s %s%s"`, command.Method, pathPrefix, command.Path),
	}
	if len(params) > 0 {
		helpLines = append(helpLines, `          echo ""`, `          echo "Flags:"`)
		for _, p := range params {
			req := ""
			if p.Required {
				req = "  (required)"
			}
			helpLines = append(helpLines, fmt.Sprintf(`          echo "  --%s <%s>%s"`, p.Name, p.Type, req))
			if p.SchemaRef != "" {
			helpLines = append(helpLines, fmt.Sprintf(`          echo "    schema: schema/%s.json"`, p.SchemaRef))
			}
		}
	}
	if command.SchemaName != "" {
		helpLines = append(helpLines,
			`          echo ""`,
			fmt.Sprintf(`          echo "  Schema: schema/%s.json"`, command.SchemaName),
		)
	}
	helpLines = append(helpLines, `          exit 0`)

	var validationBlock string
	var required []Param
	for _, p := range params {
		if p.Required {
			required = append(required, p)
		}
	}
	if len(required) > 0 {
		vLines := []string{`    _missing=""`}
		for _, p := range required {
			vLines = append(vLines, fmt.Sprintf(`    [ -z "%s" ] && _missing="$_missing %s"`, p.BashVar, p.Flag))
		}
		vLines = append(vLines,
			`    if [ -n "$_missing" ]; then`,
			`      echo "${_RED}error: missing required flags:$_missing${_RESET}" >&2`,

			fmt.Sprintf(`      echo "run '$(basename "${BASH_SOURCE[0]}") %s %s --help' for usage" >&2`, command.Group, command.Sub),
			`      exit 1`,
			`    fi`,
		)
		validationBlock = "\n" + strings.Join(vLines, "\n")
	}

	return fmt.Sprintf(`%s    while [ $# -gt 0 ]; do
      case "$1" in
%s
        --help|-h)
%s
          ;;
        *) echo "${_RED}unknown flag: $1${_RESET}" >&2; exit 1 ;;
      esac
    done%s`,
		init,
		strings.Join(flagCases, "\n"),
		strings.Join(helpLines, "\n"),
		validationBlock)
}

func urlConstruction(command Command, pathPrefix string) string {
	urlExpression := "${BASE_URL}" + pathPrefix + command.Path
	for _, param := range command.PathParams {
		urlExpression = strings.ReplaceAll(urlExpression, "{"+param.Name+"}", "${"+param.BashVar+"}")
	}
	return fmt.Sprintf(`    _url="%s"`, urlExpression)
}

func queryStringAppend(queryParams []Param) string {
	if len(queryParams) == 0 {
		return ""
	}
	lines := []string{`    _qs=""`}
	for _, p := range queryParams {
		lines = append(lines, fmt.Sprintf(`    [ -n "$%s" ] && _qs="${_qs}&%s=$(jq -rn --arg v "${%s}" '$v | @json | @uri')"`, p.BashVar, p.Name, p.BashVar))
	}
	lines = append(lines, `    [ -n "$_qs" ] && _url="${_url}?${_qs#&}"`)
	return strings.Join(lines, "\n")
}

func bodyConstruction(bodyParams []Param) string {
	if len(bodyParams) == 0 {
		return ""
	}
	return renderJqBodyStatement(bodyParams)
}

func curlInvocation(command Command) string {
	parts := []string{fmt.Sprintf(`    curl -sS --fail-with-body -X %s "$_url"`, command.Method)}
	if len(command.BodyParams) > 0 {
		parts = append(parts, `      -H "Content-Type: application/json"`)
	}
	if command.HasBearer {
		parts = append(parts, `      -H "Authorization: Bearer ${TOKEN}"`)
	}
	if len(command.BodyParams) > 0 {
		parts = append(parts, `      -d "$_body"`)
	}
	return strings.Join(parts, " \\\n") + ` || { echo "${_RED}error: request failed${_RESET}" >&2; exit 1; }`
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

func anyCommandNeedsJq(commands []Command) bool {
	for _, command := range commands {
		if len(command.BodyParams) > 0 || len(command.QueryParams) > 0 {
			return true
		}
	}
	return false
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

