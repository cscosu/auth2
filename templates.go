package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"regexp"
	"strings"
	"text/template"
)

//go:embed templates/*.html.tpl
var templateAssets embed.FS

type TemplateLoader struct {
	fs       embed.FS
	cache    map[string]*template.Template
	deps     map[string][]string
	patterns *templatePatterns
}

type templatePatterns struct {
	extends *regexp.Regexp
}

func newTemplatePatterns() *templatePatterns {
	return &templatePatterns{
		extends: regexp.MustCompile(`{{\s*template\s*"([^"]+)".*}}`),
	}
}

func NewTemplateLoader(fs embed.FS) (*TemplateLoader, error) {
	loader := &TemplateLoader{
		fs:       fs,
		cache:    make(map[string]*template.Template),
		deps:     make(map[string][]string),
		patterns: newTemplatePatterns(),
	}

	if err := loader.buildDependencyGraph(); err != nil {
		return nil, fmt.Errorf("failed to build dependency graph: %w", err)
	}

	if err := loader.parseTemplates(); err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	return loader, nil
}

func (tl *TemplateLoader) readFile(name string) (string, error) {
	content, err := tl.fs.ReadFile("templates/" + name)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (tl *TemplateLoader) findDependencies(content string) []string {
	matches := tl.patterns.extends.FindAllStringSubmatch(content, -1)
	deps := make([]string, 0)
	for _, match := range matches {
		if len(match) > 1 {
			deps = append(deps, match[1])
		}
	}
	return deps
}

func (tl *TemplateLoader) buildDependencyGraph() error {
	// First, discover all templates
	var templates []string
	err := fs.WalkDir(tl.fs, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			templateName := strings.TrimPrefix(path, "templates/")
			templates = append(templates, templateName)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Build dependency graph
	for _, tmpl := range templates {
		content, err := tl.readFile(tmpl)
		if err != nil {
			return err
		}

		deps := tl.findDependencies(content)
		tl.deps[tmpl] = deps
	}

	// Verify no circular dependencies
	for _, tmpl := range templates {
		visited := make(map[string]bool)
		if err := tl.checkCircularDeps(tmpl, visited); err != nil {
			return err
		}
	}

	return nil
}

func (tl *TemplateLoader) checkCircularDeps(tmpl string, visited map[string]bool) error {
	if visited[tmpl] {
		return fmt.Errorf("circular dependency detected for template: %s", tmpl)
	}

	visited[tmpl] = true
	for _, dep := range tl.deps[tmpl] {
		if err := tl.checkCircularDeps(dep, visited); err != nil {
			return err
		}
	}
	visited[tmpl] = false

	return nil
}

func (tl *TemplateLoader) getTemplateFiles(name string) []string {
	files := []string{name}
	visited := make(map[string]bool)

	var getDeps func(string)
	getDeps = func(tmpl string) {
		if visited[tmpl] {
			return
		}
		visited[tmpl] = true

		for _, dep := range tl.deps[tmpl] {
			files = append(files, dep)
			getDeps(dep)
		}
	}

	getDeps(name)
	return files
}

func (tl *TemplateLoader) parseTemplates() error {
	for templateName := range tl.deps {
		files := tl.getTemplateFiles(templateName)

		for i, file := range files {
			files[i] = "templates/" + file
		}

		tmpl, err := template.ParseFS(tl.fs, files...)
		if err != nil {
			return fmt.Errorf("failed to parse template %s: %w", templateName, err)
		}

		tl.cache[templateName] = tmpl
	}
	return nil
}

func (tl *TemplateLoader) ExecuteTemplate(w http.ResponseWriter, templateName string, data interface{}) error {
	tmpl, ok := tl.cache[templateName]
	if !ok {
		return fmt.Errorf("template \"%s\" not found", templateName)
	}

	return tmpl.ExecuteTemplate(w, templateName, data)
}

var Templates *TemplateLoader

func init() {
	var err error
	Templates, err = NewTemplateLoader(templateAssets)
	if err != nil {
		panic(err)
	}
}
