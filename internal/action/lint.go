package action

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ebogdum/keramos/v3/internal/engine"
	"github.com/ebogdum/keramos/v3/internal/layer"
	"github.com/ebogdum/keramos/v3/internal/values"
	"gopkg.in/yaml.v3"
)

// LintResult holds the results of linting a package.
type LintResult struct {
	Errors   []LintMessage
	Warnings []LintMessage
}

// LintMessage describes a single lint finding.
type LintMessage struct {
	Severity string // "error" or "warning"
	File     string
	Message  string
}

// IsValid returns true if no errors were found.
func (r *LintResult) IsValid() bool {
	return 0 == len(r.Errors)
}

var semverRegex = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)` +
	`(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?` +
	`(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

// Lint validates a keramos package at the given path.
func Lint(packagePath string, valueFiles []string, sets []string, profile string, strict bool) (*LintResult, error) {
	absPath, err := filepath.Abs(packagePath)
	if nil != err {
		return nil, fmt.Errorf("failed to resolve package path: %w", err)
	}

	result := &LintResult{}

	lintKeramosYAML(absPath, result)
	lintValuesYAML(absPath, result)
	lintSchema(absPath, result)
	lintTemplatesExist(absPath, result)

	if !result.IsValid() {
		return result, nil
	}

	lintRender(absPath, valueFiles, sets, profile, result)
	lintBase(absPath, result)
	lintProfile(absPath, profile, result)
	lintDuplicateTemplates(absPath, result)

	return result, nil
}

func lintKeramosYAML(absPath string, result *LintResult) {
	keramosPath := filepath.Join(absPath, "keramos.yaml")
	data, err := os.ReadFile(keramosPath)
	if nil != err {
		result.Errors = append(result.Errors, LintMessage{
			Severity: "error",
			File:     "keramos.yaml",
			Message:  "keramos.yaml not found or unreadable",
		})
		return
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); nil != err {
		result.Errors = append(result.Errors, LintMessage{
			Severity: "error",
			File:     "keramos.yaml",
			Message:  fmt.Sprintf("keramos.yaml is not valid YAML: %s", err.Error()),
		})
		return
	}

	apiVersion, _ := raw["apiVersion"].(string)
	name, _ := raw["name"].(string)
	version, _ := raw["version"].(string)

	if "" == apiVersion {
		result.Errors = append(result.Errors, LintMessage{
			Severity: "error",
			File:     "keramos.yaml",
			Message:  "apiVersion is required",
		})
	} else if "keramos/v1" != apiVersion {
		result.Errors = append(result.Errors, LintMessage{
			Severity: "error",
			File:     "keramos.yaml",
			Message:  fmt.Sprintf("apiVersion must be \"keramos/v1\", got %q", apiVersion),
		})
	}

	if "" == name {
		result.Errors = append(result.Errors, LintMessage{
			Severity: "error",
			File:     "keramos.yaml",
			Message:  "name is required",
		})
	}

	if "" == version {
		result.Errors = append(result.Errors, LintMessage{
			Severity: "error",
			File:     "keramos.yaml",
			Message:  "version is required",
		})
	} else if !semverRegex.MatchString(version) {
		result.Errors = append(result.Errors, LintMessage{
			Severity: "error",
			File:     "keramos.yaml",
			Message:  fmt.Sprintf("version %q is not valid semver", version),
		})
	}
}

func lintValuesYAML(absPath string, result *LintResult) {
	valuesPath := filepath.Join(absPath, "values.yaml")
	data, err := os.ReadFile(valuesPath)
	if nil != err {
		if os.IsNotExist(err) {
			return
		}
		result.Errors = append(result.Errors, LintMessage{
			Severity: "error",
			File:     "values.yaml",
			Message:  fmt.Sprintf("cannot read values.yaml: %s", err.Error()),
		})
		return
	}

	var vals any
	if err := yaml.Unmarshal(data, &vals); nil != err {
		result.Errors = append(result.Errors, LintMessage{
			Severity: "error",
			File:     "values.yaml",
			Message:  fmt.Sprintf("values.yaml is not valid YAML: %s", err.Error()),
		})
	}
}

func lintSchema(absPath string, result *LintResult) {
	schemaPath := filepath.Join(absPath, "values.schema.json")
	data, err := os.ReadFile(schemaPath)
	if nil != err {
		if os.IsNotExist(err) {
			return
		}
		result.Errors = append(result.Errors, LintMessage{
			Severity: "error",
			File:     "values.schema.json",
			Message:  fmt.Sprintf("cannot read values.schema.json: %s", err.Error()),
		})
		return
	}

	var schema any
	if err := json.Unmarshal(data, &schema); nil != err {
		result.Errors = append(result.Errors, LintMessage{
			Severity: "error",
			File:     "values.schema.json",
			Message:  fmt.Sprintf("values.schema.json is not valid JSON: %s", err.Error()),
		})
	}
}

func lintTemplatesExist(absPath string, result *LintResult) {
	templatesDir := filepath.Join(absPath, "templates")
	entries, err := os.ReadDir(templatesDir)
	if nil != err {
		if os.IsNotExist(err) {
			result.Warnings = append(result.Warnings, LintMessage{
				Severity: "warning",
				File:     "templates/",
				Message:  "templates/ directory not found",
			})
			return
		}
		result.Errors = append(result.Errors, LintMessage{
			Severity: "error",
			File:     "templates/",
			Message:  fmt.Sprintf("cannot read templates/ directory: %s", err.Error()),
		})
		return
	}

	hasYAML := false
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && !strings.HasPrefix(name, "_") && (strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")) {
			hasYAML = true
			break
		}
	}
	if !hasYAML {
		result.Warnings = append(result.Warnings, LintMessage{
			Severity: "warning",
			File:     "templates/",
			Message:  "templates/ contains no .yaml files",
		})
	}
}

func lintRender(absPath string, valueFiles []string, sets []string, profile string, result *LintResult) {
	resolved, err := layer.Resolve(absPath, profile)
	if nil != err {
		result.Errors = append(result.Errors, LintMessage{
			Severity: "error",
			File:     "",
			Message:  fmt.Sprintf("failed to resolve package: %s", err.Error()),
		})
		return
	}

	mergedValues, err := values.Resolve(map[string]any(resolved.Values), valueFiles, sets)
	if nil != err {
		result.Errors = append(result.Errors, LintMessage{
			Severity: "error",
			File:     "",
			Message:  fmt.Sprintf("failed to resolve values: %s", err.Error()),
		})
		return
	}

	ctx := &engine.RenderContext{
		Values: mergedValues,
		Package: map[string]any{
			"name":       resolved.Metadata.Name,
			"version":    resolved.Metadata.Version,
			"appVersion": resolved.Metadata.AppVersion,
		},
		Release: map[string]any{
			"name":      resolved.Metadata.Name,
			"namespace": "default",
			"revision":  1,
			"isUpgrade": false,
			"isInstall": true,
		},
		Capabilities: map[string]any{},
		Files:        resolved.Files,
	}

	eng := engine.New()
	_, err = eng.Render(resolved.Templates, resolved.Partials, ctx)
	if nil != err {
		result.Errors = append(result.Errors, LintMessage{
			Severity: "error",
			File:     "",
			Message:  fmt.Sprintf("template rendering failed: %s", err.Error()),
		})
	}
}

func lintBase(absPath string, result *LintResult) {
	keramosPath := filepath.Join(absPath, "keramos.yaml")
	data, err := os.ReadFile(keramosPath)
	if nil != err {
		return
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); nil != err {
		return
	}

	base, _ := raw["base"].(string)
	if "" == base {
		return
	}

	basePath := filepath.Join(absPath, base)
	info, err := os.Stat(basePath)
	if nil != err || !info.IsDir() {
		result.Errors = append(result.Errors, LintMessage{
			Severity: "error",
			File:     "keramos.yaml",
			Message:  fmt.Sprintf("base %q does not exist or is not a directory", base),
		})
		return
	}

	baseKeramos := filepath.Join(basePath, "keramos.yaml")
	if _, err := os.Stat(baseKeramos); nil != err {
		result.Errors = append(result.Errors, LintMessage{
			Severity: "error",
			File:     "keramos.yaml",
			Message:  fmt.Sprintf("base %q has no keramos.yaml", base),
		})
	}
}

func lintProfile(absPath string, profile string, result *LintResult) {
	if "" == profile {
		return
	}

	profileDir := filepath.Join(absPath, "profiles", profile)
	info, err := os.Stat(profileDir)
	if nil != err || !info.IsDir() {
		result.Errors = append(result.Errors, LintMessage{
			Severity: "error",
			File:     "",
			Message:  fmt.Sprintf("profile %q not found in profiles/", profile),
		})
	}
}

func lintDuplicateTemplates(absPath string, result *LintResult) {
	keramosPath := filepath.Join(absPath, "keramos.yaml")
	data, err := os.ReadFile(keramosPath)
	if nil != err {
		return
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); nil != err {
		return
	}

	base, _ := raw["base"].(string)
	if "" == base {
		return
	}

	basePath := filepath.Join(absPath, base, "templates")
	overlayPath := filepath.Join(absPath, "templates")

	baseNames, err := templateNames(basePath)
	if nil != err {
		return
	}
	overlayNames, err := templateNames(overlayPath)
	if nil != err {
		return
	}

	for name := range overlayNames {
		if baseNames[name] {
			result.Warnings = append(result.Warnings, LintMessage{
				Severity: "warning",
				File:     name,
				Message:  fmt.Sprintf("template %q overrides base template", name),
			})
		}
	}
}

func templateNames(dir string) (map[string]bool, error) {
	names := make(map[string]bool)
	entries, err := os.ReadDir(dir)
	if nil != err {
		if os.IsNotExist(err) {
			return names, nil
		}
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			names[entry.Name()] = true
		}
	}
	return names, nil
}
