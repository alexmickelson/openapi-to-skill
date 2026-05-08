package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alexmickelson/openapi-to-skill/internal/openapi"
)

func WriteSkill(outDir, name string, doc *openapi.Document) (string, error) {
	hasBearer := HasBearerAuth(doc)
	commands := ExtractCommands(doc, hasBearer)
	skillPath := filepath.Join(outDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte(renderSkill(name, doc, commands, hasBearer)), 0o644); err != nil {
		return "", err
	}
	return skillPath, nil
}

func renderSkill(name string, doc *openapi.Document, commands []Command, hasBearer bool) string {
	title := titleFromDoc(doc, name)
	description := descriptionFromDoc(doc, title)
	var builder strings.Builder
	writeFrontmatter(&builder, name, description)
	writeSkillHeader(&builder, name, title)
	writeCommandTable(&builder, commands)
	if hasBearer {
		writeBearerAuthNote(&builder, envVarPrefix(name))
	}
	lineWriter(&builder)("")
	return builder.String()
}

func writeFrontmatter(builder *strings.Builder, name, description string) {
	writeLine := lineWriter(builder)
	writeLine("---")
	writeLine(fmt.Sprintf("name: %s", name))
	writeLine(fmt.Sprintf("description: '%s'", description))
	writeLine("---")
	writeLine("")
}

func writeSkillHeader(builder *strings.Builder, name, title string) {
	writeLine := lineWriter(builder)
	writeLine(fmt.Sprintf("# %s", title))
	writeLine("")
	writeLine(fmt.Sprintf("CLI: [./scripts/%s](./scripts/%s)", name, name))
	writeLine("")
}

func writeCommandTable(builder *strings.Builder, commands []Command) {
	writeLine := lineWriter(builder)
	writeLine("| Command | Flags |")
	writeLine("| --- | --- |")
	for _, command := range commands {
		commandCell := fmt.Sprintf("`%s %s`", command.Group, command.Sub)
		writeLine(fmt.Sprintf("| %s | %s |", commandCell, commandFlagsCell(command)))
	}
}

func writeBearerAuthNote(builder *strings.Builder, envPrefix string) {
	writeLine := lineWriter(builder)
	writeLine("")
	writeLine(fmt.Sprintf("Set `%s_TOKEN` for bearer auth.", envPrefix))
}

func commandFlagsCell(command Command) string {
	var flagTokens []string
	for _, param := range append(append(command.PathParams, command.QueryParams...), command.BodyParams...) {
		flagTokens = append(flagTokens, fmt.Sprintf("`%s`", param.Flag))
	}
	flagsCell := strings.Join(flagTokens, " ")
	if command.SchemaName == "" {
		return flagsCell
	}
	schemaLink := fmt.Sprintf("[schema](./schema/%s.json)", command.SchemaName)
	if flagsCell != "" {
		return flagsCell + " · " + schemaLink
	}
	return schemaLink
}

func titleFromDoc(doc *openapi.Document, fallback string) string {
	if doc.Info.Title != "" {
		return doc.Info.Title
	}
	return fallback
}

func descriptionFromDoc(doc *openapi.Document, title string) string {
	description := doc.Info.Description
	if description == "" {
		description = fmt.Sprintf("Manage %s resources via CLI.", title)
	}
	if newlineIndex := strings.IndexByte(description, '\n'); newlineIndex >= 0 {
		description = description[:newlineIndex]
	}
	return strings.ReplaceAll(description, "'", "''")
}

