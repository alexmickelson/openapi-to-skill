package generator

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/alexmickelson/openapi-to-skill/internal/openapi"
)

func WriteSchemas(outDir string, doc *openapi.Document) ([]string, error) {
	var writtenPaths []string
	for schemaName, schema := range doc.Components.Schemas {
		data, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			return nil, err
		}
		schemaPath := filepath.Join(outDir, "schema", schemaName+".json")
		if err := os.WriteFile(schemaPath, data, 0o644); err != nil {
			return nil, err
		}
		writtenPaths = append(writtenPaths, schemaPath)
	}
	return writtenPaths, nil
}

func WriteInlineSchemas(outDir string, commands []Command) ([]string, error) {
	var writtenPaths []string
	seen := map[string]bool{}
	for _, cmd := range commands {
		for _, p := range cmd.BodyParams {
			if p.SchemaJSON == "" || seen[p.SchemaRef] {
				continue
			}
			seen[p.SchemaRef] = true
			schemaPath := filepath.Join(outDir, "schema", p.SchemaRef+".json")
			if err := os.WriteFile(schemaPath, []byte(p.SchemaJSON), 0o644); err != nil {
				return nil, err
			}
			writtenPaths = append(writtenPaths, schemaPath)
		}
	}
	return writtenPaths, nil
}

