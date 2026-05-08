package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/alexmickelson/openapi-to-skill/internal/generator"
	"github.com/alexmickelson/openapi-to-skill/internal/openapi"
)

func Execute() {
	nameOverride, force := parseFlags()
	outputDir, specSource := parsePositionalArgs()

	doc := loadSpec(specSource)
	projectName := resolveProjectName(nameOverride, doc.Info.Title)

	guardOutputDir(outputDir, force)
	createOutputDirs(outputDir)

	writtenPaths := runGenerators(outputDir, projectName, doc)
	for _, writtenPath := range writtenPaths {
		fmt.Println(writtenPath)
	}
}

func parseFlags() (nameOverride string, force bool) {
	flag.StringVar(&nameOverride, "name", "", "override the derived project name")
	flag.BoolVar(&force, "force", false, "overwrite an existing skill directory")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: openapi-to-skill [--name NAME] [--force] <output-dir> <openapi-url>")
		flag.PrintDefaults()
	}
	flag.Parse()
	return nameOverride, force
}

func parsePositionalArgs() (outputDir, specSource string) {
	args := flag.Args()
	if len(args) != 2 {
		flag.Usage()
		os.Exit(1)
	}
	return args[0], args[1]
}

func loadSpec(specSource string) *openapi.Document {
	data, err := openapi.Fetch(specSource)
	if err != nil {
		fatalf("fetch: %v", err)
	}
	doc, err := openapi.Parse(data)
	if err != nil {
		fatalf("parse: %v", err)
	}
	return doc
}

func resolveProjectName(nameOverride, titleFromSpec string) string {
	if nameOverride != "" {
		return nameOverride
	}
	return generator.ProjectName(titleFromSpec)
}

func guardOutputDir(outputDir string, force bool) {
	info, err := os.Stat(outputDir)
	if err == nil && info.IsDir() && !force {
		fatalf("output directory %q already exists; use --force to overwrite", outputDir)
	}
}

func createOutputDirs(outputDir string) {
	dirs := []string{
		outputDir,
		filepath.Join(outputDir, "scripts"),
		filepath.Join(outputDir, "schema"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fatalf("mkdir %s: %v", dir, err)
		}
	}
}

func runGenerators(outputDir, projectName string, doc *openapi.Document) []string {
	schemaFiles, err := generator.WriteSchemas(outputDir, doc)
	if err != nil {
		fatalf("schemas: %v", err)
	}

	scriptFile, err := generator.WriteScript(outputDir, projectName, doc)
	if err != nil {
		fatalf("script: %v", err)
	}

	skillFile, err := generator.WriteSkill(outputDir, projectName, doc)
	if err != nil {
		fatalf("skill: %v", err)
	}

	return append(schemaFiles, scriptFile, skillFile)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "openapi-to-skill: "+format+"\n", args...)
	os.Exit(1)
}

