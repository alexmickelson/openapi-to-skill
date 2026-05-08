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

