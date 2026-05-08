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
	parts := []string{
		skillFrontmatter(name, description),
		skillHeader(name, title),
		commandList(name, commands),
	}
	if hasBearer {
		parts = append(parts, fmt.Sprintf("Set `%s_TOKEN` for bearer auth.", envVarPrefix(name)))
	}
	return strings.Join(parts, "\n") + "\n"
}

func skillFrontmatter(name, description string) string {
	return fmt.Sprintf("---\nname: %s\ndescription: '%s'\n---", name, description)
}

func skillHeader(name, title string) string {
	return fmt.Sprintf("# %s\n\nCLI: [./scripts/%s](./scripts/%s)", title, name, name)
}

func commandList(name string, commands []Command) string {
	lines := []string{
		fmt.Sprintf("Use `--help` at any level to discover more: `%s --help`, `%s <group> --help`, `%s <group> <sub> --help`", name, name, name),
		"",
		"## Commands",
		"",
	}
	for _, cmd := range commands {
		lines = append(lines, fmt.Sprintf("- `%s %s`", cmd.Group, cmd.Sub))
	}
	return strings.Join(lines, "\n")
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

