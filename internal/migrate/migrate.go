package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	keramoserr "github.com/ebogdum/keramos/internal/errors"
	"gopkg.in/yaml.v3"
)

// MigrateResult holds the result of migrating a Helm chart.
type MigrateResult struct {
	PackagePath    string
	ConvertedFiles []string
	ManualReview   []ReviewItem
	Warnings       []string
}

// ReviewItem marks a template line that needs human attention.
type ReviewItem struct {
	File     string
	Line     int
	Reason   string
	Original string
}

// helmChart represents a parsed Chart.yaml.
type helmChart struct {
	APIVersion   string                   `yaml:"apiVersion"`
	Name         string                   `yaml:"name"`
	Version      string                   `yaml:"version"`
	AppVersion   string                   `yaml:"appVersion,omitempty"`
	Description  string                   `yaml:"description,omitempty"`
	Type         string                   `yaml:"type,omitempty"`
	KubeVersion  string                   `yaml:"kubeVersion,omitempty"`
	Dependencies []helmDep                `yaml:"dependencies,omitempty"`
	Maintainers  []map[string]string      `yaml:"maintainers,omitempty"`
	Keywords     []string                 `yaml:"keywords,omitempty"`
	Annotations  map[string]string        `yaml:"annotations,omitempty"`
	Sources      []string                 `yaml:"sources,omitempty"`
	Home         string                   `yaml:"home,omitempty"`
	Icon         string                   `yaml:"icon,omitempty"`
	Deprecated   bool                     `yaml:"deprecated,omitempty"`
	Extra        map[string]any   `yaml:",inline"`
	RawEntries   []map[string]any `yaml:"-"`
}

type helmDep struct {
	Name       string   `yaml:"name"`
	Version    string   `yaml:"version"`
	Repository string   `yaml:"repository"`
	Condition  string   `yaml:"condition,omitempty"`
	Tags       []string `yaml:"tags,omitempty"`
	Alias      string   `yaml:"alias,omitempty"`
	Enabled    *bool    `yaml:"enabled,omitempty"`
}

type keramosMeta struct {
	APIVersion   string            `yaml:"apiVersion"`
	Name         string            `yaml:"name"`
	Version      string            `yaml:"version"`
	AppVersion   string            `yaml:"appVersion,omitempty"`
	Description  string            `yaml:"description,omitempty"`
	Type         string            `yaml:"type,omitempty"`
	KubeVersion  string            `yaml:"kubeVersion,omitempty"`
	Dependencies []keramosDep         `yaml:"dependencies,omitempty"`
	Maintainers  []keramosMaintainer  `yaml:"maintainers,omitempty"`
	Keywords     []string          `yaml:"keywords,omitempty"`
	Annotations  map[string]string `yaml:"annotations,omitempty"`
}

type keramosDep struct {
	Name       string   `yaml:"name"`
	Version    string   `yaml:"version"`
	Repository string   `yaml:"repository"`
	Condition  string   `yaml:"condition,omitempty"`
	Tags       []string `yaml:"tags,omitempty"`
	Alias      string   `yaml:"alias,omitempty"`
	Enabled    *bool    `yaml:"enabled,omitempty"`
}

type keramosMaintainer struct {
	Name  string `yaml:"name"`
	Email string `yaml:"email,omitempty"`
}

// Migrate converts a Helm chart directory to a keramos package.
// When dryRun is true, everything is parsed and validated but no files are written.
// The dryRun parameter is optional; if omitted it defaults to false.
func Migrate(chartPath, outputDir string, strict bool, optDryRun ...bool) (*MigrateResult, error) {
	dryRun := len(optDryRun) > 0 && optDryRun[0]
	chartYAMLPath := filepath.Join(chartPath, "Chart.yaml")
	if _, err := os.Stat(chartYAMLPath); nil != err {
		return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "not a Helm chart: Chart.yaml not found in %s", chartPath)
	}

	chartData, err := os.ReadFile(chartYAMLPath)
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrParse, "cannot read Chart.yaml", err)
	}

	var chart helmChart
	if err := yaml.Unmarshal(chartData, &chart); nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrParse, "invalid Chart.yaml", err)
	}

	if "" == chart.Name {
		return nil, keramoserr.NewError(keramoserr.ErrCLIValidation, "Chart.yaml missing required field: name")
	}

	if "" == outputDir {
		outputDir = chart.Name + "-keramos"
	}

	result := &MigrateResult{
		PackagePath:    outputDir,
		ConvertedFiles: make([]string, 0, 16),
		ManualReview:   make([]ReviewItem, 0),
		Warnings:       make([]string, 0),
	}

	if dryRun {
		return migrateDryRun(chartPath, result, strict)
	}

	// Write to a temp directory first, then rename atomically on success.
	parentDir := filepath.Dir(outputDir)
	if mkErr := os.MkdirAll(parentDir, 0o755); nil != mkErr {
		return nil, keramoserr.WrapError(keramoserr.ErrInternal, "cannot create parent directory", mkErr)
	}

	tmpDir, tmpErr := os.MkdirTemp(parentDir, ".keramos-migrate-*")
	if nil != tmpErr {
		return nil, keramoserr.WrapError(keramoserr.ErrInternal, "cannot create temp directory", tmpErr)
	}

	writeErr := migrateWriteAll(chartPath, &chart, tmpDir, result)
	if nil != writeErr {
		_ = os.RemoveAll(tmpDir)
		return nil, writeErr
	}

	// In strict mode, refuse to commit the output if any items need manual review.
	if strict && len(result.ManualReview) > 0 {
		_ = os.RemoveAll(tmpDir)
		return result, keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "strict mode: %d items require manual review", len(result.ManualReview))
	}

	// Refuse to clobber an existing non-empty outputDir unless the
	// caller explicitly opts in with KERAMOS_MIGRATE_FORCE=1. Without this
	// guard, `keramos migrate ./chart -d ./mypkg` silently deleted any
	// hand-edited package the user had at ./mypkg before swapping in
	// the freshly-migrated tree — a quiet data loss.
	if outputExists, oeErr := dirHasContent(outputDir); nil != oeErr {
		_ = os.RemoveAll(tmpDir)
		return nil, oeErr
	} else if outputExists && "1" != os.Getenv("KERAMOS_MIGRATE_FORCE") {
		_ = os.RemoveAll(tmpDir)
		return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
			"refusing to overwrite non-empty output directory %s; remove it first or set KERAMOS_MIGRATE_FORCE=1",
			outputDir)
	}

	// Remove existing outputDir if present, then atomically rename.
	_ = os.RemoveAll(outputDir)
	if renameErr := os.Rename(tmpDir, outputDir); nil != renameErr {
		_ = os.RemoveAll(tmpDir)
		return nil, keramoserr.WrapError(keramoserr.ErrInternal, "cannot rename temp directory to output", renameErr)
	}

	return result, nil
}

// dirHasContent reports whether path exists and contains anything.
// Returns (false, nil) if the path doesn't exist.
func dirHasContent(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if nil != err {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, keramoserr.WrapErrorf(keramoserr.ErrInternal, err, "stat %s", path)
	}
	return len(entries) > 0, nil
}

// migrateWriteAll performs all file writes into the given directory.
func migrateWriteAll(chartPath string, chart *helmChart, writeDir string, result *MigrateResult) error {
	if mkErr := os.MkdirAll(writeDir, 0o755); nil != mkErr {
		return keramoserr.WrapError(keramoserr.ErrInternal, "cannot create output directory", mkErr)
	}

	if err := convertChartYAML(chart, writeDir, result); nil != err {
		return err
	}

	if err := copyFileIfExists(filepath.Join(chartPath, "values.yaml"), filepath.Join(writeDir, "values.yaml"), result); nil != err {
		return err
	}
	if err := copyFileIfExists(filepath.Join(chartPath, "values.schema.json"), filepath.Join(writeDir, "values.schema.json"), result); nil != err {
		return err
	}

	templatesDir := filepath.Join(chartPath, "templates")
	if _, statErr := os.Stat(templatesDir); nil == statErr {
		if err := convertTemplates(templatesDir, writeDir, result); nil != err {
			return err
		}
	}

	notesPath := filepath.Join(chartPath, "templates", "NOTES.txt")
	if err := convertNotes(notesPath, writeDir, result); nil != err {
		return err
	}

	// Copy crds/ verbatim (helm convention preserved by keramos's --include-crds).
	if err := copyCRDsDir(chartPath, writeDir, result); nil != err {
		return err
	}

	if err := generateKeramosIgnore(writeDir, result); nil != err {
		return err
	}

	return nil
}

// copyCRDsDir copies <chartPath>/crds/ to <writeDir>/crds/ verbatim. CRDs are
// not templated and pass through unchanged.
func copyCRDsDir(chartPath, writeDir string, result *MigrateResult) error {
	src := filepath.Join(chartPath, "crds")
	info, err := os.Stat(src)
	if nil != err {
		if os.IsNotExist(err) {
			return nil
		}
		return keramoserr.WrapError(keramoserr.ErrInternal, "cannot stat crds directory", err)
	}
	if !info.IsDir() {
		return nil
	}
	dst := filepath.Join(writeDir, "crds")
	cleanDst := filepath.Clean(dst) + string(filepath.Separator)
	walkErr := filepath.Walk(src, func(path string, fi os.FileInfo, e error) error {
		if nil != e {
			return e
		}
		// Refuse symlinks — a malicious chart could otherwise read host files.
		if lstat, lerr := os.Lstat(path); nil == lerr && 0 != lstat.Mode()&os.ModeSymlink {
			return nil
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		// Verify the resolved target stays inside the destination directory.
		if !strings.HasPrefix(filepath.Clean(target)+string(filepath.Separator), cleanDst) {
			return keramoserr.NewErrorf(keramoserr.ErrInternal,
				"crd path escapes output directory: %s", target)
		}
		if fi.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, readErr := os.ReadFile(path)
		if nil != readErr {
			return keramoserr.WrapErrorf(keramoserr.ErrInternal, readErr, "cannot read CRD %s", path)
		}
		if mkErr := os.MkdirAll(filepath.Dir(target), 0o755); nil != mkErr {
			return mkErr
		}
		if writeErr := os.WriteFile(target, data, 0o644); nil != writeErr {
			return keramoserr.WrapErrorf(keramoserr.ErrInternal, writeErr, "cannot write CRD %s", target)
		}
		result.ConvertedFiles = append(result.ConvertedFiles, filepath.Join("crds", rel))
		return nil
	})
	if nil != walkErr {
		return walkErr
	}
	return nil
}

// migrateDryRun validates and converts everything without writing to disk.
func migrateDryRun(chartPath string, result *MigrateResult, strict bool) (*MigrateResult, error) {
	// Simulate convertChartYAML by building the metadata (validates structure).
	result.ConvertedFiles = append(result.ConvertedFiles, "keramos.yaml")

	// Check optional files.
	if _, statErr := os.Stat(filepath.Join(chartPath, "values.yaml")); nil == statErr {
		result.ConvertedFiles = append(result.ConvertedFiles, "values.yaml")
	}
	if _, statErr := os.Stat(filepath.Join(chartPath, "values.schema.json")); nil == statErr {
		result.ConvertedFiles = append(result.ConvertedFiles, "values.schema.json")
	}

	// Parse and convert templates in memory to detect review items.
	templatesDir := filepath.Join(chartPath, "templates")
	if _, statErr := os.Stat(templatesDir); nil == statErr {
		if err := dryRunConvertTemplates(templatesDir, result); nil != err {
			return nil, err
		}
	}

	result.ConvertedFiles = append(result.ConvertedFiles, ".keramosignore")

	if strict && len(result.ManualReview) > 0 {
		return result, keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "strict mode: %d items require manual review", len(result.ManualReview))
	}

	return result, nil
}

// dryRunConvertTemplates reads and converts templates in memory without writing.
func dryRunConvertTemplates(templatesDir string, result *MigrateResult) error {
	entries, err := os.ReadDir(templatesDir)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrInternal, "cannot read templates directory", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		srcPath := filepath.Join(templatesDir, name)

		if entry.IsDir() {
			if "tests" == name {
				if testErr := dryRunConvertTestTemplates(srcPath, result); nil != testErr {
					return testErr
				}
			}
			continue
		}

		if "NOTES.txt" == name {
			result.ConvertedFiles = append(result.ConvertedFiles, filepath.Join("templates", "notes.yaml"))
			continue
		}

		if "_helpers.tpl" == name {
			result.ConvertedFiles = append(result.ConvertedFiles, filepath.Join("templates", "_helpers.yaml"))
			data, readErr := os.ReadFile(srcPath)
			if nil != readErr {
				return keramoserr.WrapErrorf(keramoserr.ErrInternal, readErr, "cannot read template %s", name)
			}
			convertTemplateContent(string(data), "_helpers.tpl", result)
			continue
		}

		data, readErr := os.ReadFile(srcPath)
		if nil != readErr {
			return keramoserr.WrapErrorf(keramoserr.ErrInternal, readErr, "cannot read template %s", name)
		}

		content := string(data)
		if isHookTemplate(content) {
			result.ConvertedFiles = append(result.ConvertedFiles, filepath.Join("hooks", name))
			stripped := stripHookAnnotation(content)
			convertTemplateContent(stripped, name, result)
			continue
		}

		convertTemplateContent(content, name, result)
		result.ConvertedFiles = append(result.ConvertedFiles, filepath.Join("templates", name))
	}

	return nil
}

// dryRunConvertTestTemplates reads and converts test templates in memory without writing.
func dryRunConvertTestTemplates(testsDir string, result *MigrateResult) error {
	entries, err := os.ReadDir(testsDir)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrInternal, "cannot read tests directory", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		srcPath := filepath.Join(testsDir, entry.Name())
		data, readErr := os.ReadFile(srcPath)
		if nil != readErr {
			return keramoserr.WrapErrorf(keramoserr.ErrInternal, readErr, "cannot read test template %s", entry.Name())
		}
		content := stripHookAnnotation(string(data))
		convertTemplateContent(content, filepath.Join("tests", entry.Name()), result)
		result.ConvertedFiles = append(result.ConvertedFiles, filepath.Join("tests", entry.Name()))
	}

	return nil
}

func convertChartYAML(chart *helmChart, outputDir string, result *MigrateResult) error {
	meta := keramosMeta{
		APIVersion:  "keramos/v1",
		Name:        chart.Name,
		Version:     chart.Version,
		AppVersion:  chart.AppVersion,
		Description: chart.Description,
		Type:        chart.Type,
		KubeVersion: chart.KubeVersion,
		Keywords:    chart.Keywords,
		Annotations: chart.Annotations,
	}

	if len(chart.Dependencies) > 0 {
		deps := make([]keramosDep, 0, len(chart.Dependencies))
		for _, d := range chart.Dependencies {
			deps = append(deps, keramosDep(d))
		}
		meta.Dependencies = deps
	}

	if len(chart.Maintainers) > 0 {
		maintainers := make([]keramosMaintainer, 0, len(chart.Maintainers))
		for _, m := range chart.Maintainers {
			maintainers = append(maintainers, keramosMaintainer{
				Name:  m["name"],
				Email: m["email"],
			})
		}
		meta.Maintainers = maintainers
	}

	data, err := yaml.Marshal(meta)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrInternal, "failed to marshal keramos.yaml", err)
	}

	dest := filepath.Join(outputDir, "keramos.yaml")
	if err := os.WriteFile(dest, data, 0o644); nil != err {
		return keramoserr.WrapError(keramoserr.ErrInternal, "failed to write keramos.yaml", err)
	}
	result.ConvertedFiles = append(result.ConvertedFiles, "keramos.yaml")
	return nil
}

func convertTemplates(templatesDir, outputDir string, result *MigrateResult) error {
	outTemplates := filepath.Join(outputDir, "templates")
	if err := os.MkdirAll(outTemplates, 0o755); nil != err {
		return keramoserr.WrapError(keramoserr.ErrInternal, "cannot create templates directory", err)
	}

	entries, err := os.ReadDir(templatesDir)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrInternal, "cannot read templates directory", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		srcPath := filepath.Join(templatesDir, name)

		if entry.IsDir() {
			if "tests" == name {
				if err := convertTestTemplates(srcPath, outputDir, result); nil != err {
					return err
				}
				continue
			}
			continue
		}

		if "NOTES.txt" == name {
			continue // handled separately
		}

		// Refuse to follow symlinks. A hostile chart with templates/x.yaml
		// pointing at /etc/passwd would otherwise embed host secrets in
		// the migrated package.
		lstat, lerr := os.Lstat(srcPath)
		if nil != lerr {
			return keramoserr.WrapErrorf(keramoserr.ErrInternal, lerr, "cannot lstat template %s", name)
		}
		if 0 != lstat.Mode()&os.ModeSymlink {
			return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
				"refusing to follow symlink in templates: %s", srcPath)
		}

		if "_helpers.tpl" == name {
			if err := convertHelpers(srcPath, outputDir, result); nil != err {
				return err
			}
			continue
		}

		data, readErr := os.ReadFile(srcPath)
		if nil != readErr {
			return keramoserr.WrapErrorf(keramoserr.ErrInternal, readErr, "cannot read template %s", name)
		}

		content := string(data)

		// Check for helm.sh/hook annotation — move hooks to hooks/ dir
		if isHookTemplate(content) {
			if err := convertHookTemplate(name, content, outputDir, result); nil != err {
				return err
			}
			continue
		}

		converted := convertTemplateContent(content, name, result)
		destPath := filepath.Join(outTemplates, name)
		if err := os.WriteFile(destPath, []byte(converted), 0o644); nil != err {
			return keramoserr.WrapErrorf(keramoserr.ErrInternal, err, "cannot write template %s", name)
		}
		result.ConvertedFiles = append(result.ConvertedFiles, filepath.Join("templates", name))
	}
	return nil
}

func convertTestTemplates(testsDir, outputDir string, result *MigrateResult) error {
	outTests := filepath.Join(outputDir, "tests")
	if err := os.MkdirAll(outTests, 0o755); nil != err {
		return keramoserr.WrapError(keramoserr.ErrInternal, "cannot create tests directory", err)
	}

	entries, err := os.ReadDir(testsDir)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrInternal, "cannot read tests directory", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		srcPath := filepath.Join(testsDir, entry.Name())
		data, readErr := os.ReadFile(srcPath)
		if nil != readErr {
			return keramoserr.WrapErrorf(keramoserr.ErrInternal, readErr, "cannot read test template %s", entry.Name())
		}

		content := stripHookAnnotation(string(data))
		converted := convertTemplateContent(content, filepath.Join("tests", entry.Name()), result)
		destPath := filepath.Join(outTests, entry.Name())
		if err := os.WriteFile(destPath, []byte(converted), 0o644); nil != err {
			return keramoserr.WrapErrorf(keramoserr.ErrInternal, err, "cannot write test template %s", entry.Name())
		}
		result.ConvertedFiles = append(result.ConvertedFiles, filepath.Join("tests", entry.Name()))
	}
	return nil
}

var hookAnnotationRe = regexp.MustCompile(`(?m)^\s*"?helm\.sh/hook"?\s*:\s*.*$\n?`)

func isHookTemplate(content string) bool {
	return hookAnnotationRe.MatchString(content)
}

func stripHookAnnotation(content string) string {
	return hookAnnotationRe.ReplaceAllString(content, "")
}

func convertHookTemplate(name, content, outputDir string, result *MigrateResult) error {
	hooksDir := filepath.Join(outputDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); nil != err {
		return keramoserr.WrapError(keramoserr.ErrInternal, "cannot create hooks directory", err)
	}

	stripped := stripHookAnnotation(content)
	converted := convertTemplateContent(stripped, name, result)
	destPath := filepath.Join(hooksDir, name)
	if err := os.WriteFile(destPath, []byte(converted), 0o644); nil != err {
		return keramoserr.WrapErrorf(keramoserr.ErrInternal, err, "cannot write hook template %s", name)
	}
	result.ConvertedFiles = append(result.ConvertedFiles, filepath.Join("hooks", name))
	return nil
}

// convertTemplateContent converts Go template expressions to keramos expressions.
// Uses block-aware conversion that handles if/else/end, with/end, range/end patterns.
func convertTemplateContent(content, filename string, result *MigrateResult) string {
	return convertTemplateContentV2(content, filename, result)
}

// Regex patterns for Helm template expressions
var (
	// Complex constructs that need manual review
	reIfBlock    = regexp.MustCompile(`\{\{-?\s*if\s+`)
	reElse       = regexp.MustCompile(`\{\{-?\s*else\s*-?\}\}`)
	reEnd        = regexp.MustCompile(`\{\{-?\s*end\s*-?\}\}`)
	reRange      = regexp.MustCompile(`\{\{-?\s*range\s+`)
	reTpl        = regexp.MustCompile(`\{\{-?\s*tpl\s+`)
	reWith       = regexp.MustCompile(`\{\{-?\s*with\s+`)

	// Include on its own line
	reIncludeLine = regexp.MustCompile(`^\s*\{\{-?\s*include\s+"([^"]+)"\s+\.\s*-?\}\}\s*$`)

	// Simple expressions: {{ .Values.x }}, {{ .Release.Name }}, etc.
	reSimpleExpr = regexp.MustCompile(`\{\{-?\s*(.+?)\s*-?\}\}`)
)

func convertLine(line, filename string, lineNum int, result *MigrateResult) string {
	// Check for include-only line first
	if m := reIncludeLine.FindStringSubmatch(line); nil != m {
		indent := extractIndent(line)
		return indent + "$include: " + m[1]
	}

	// Flag complex control flow for manual review
	if reRange.MatchString(line) {
		result.ManualReview = append(result.ManualReview, ReviewItem{
			File: filename, Line: lineNum, Reason: "range block requires manual conversion", Original: strings.TrimSpace(line),
		})
		return line
	}
	if reTpl.MatchString(line) {
		result.ManualReview = append(result.ManualReview, ReviewItem{
			File: filename, Line: lineNum, Reason: "tpl call requires manual conversion", Original: strings.TrimSpace(line),
		})
		return line
	}
	if reWith.MatchString(line) {
		result.ManualReview = append(result.ManualReview, ReviewItem{
			File: filename, Line: lineNum, Reason: "with block requires manual conversion", Original: strings.TrimSpace(line),
		})
		return line
	}

	// Flag if/else/end blocks for review (complex control flow)
	if reIfBlock.MatchString(line) || reElse.MatchString(line) || reEnd.MatchString(line) {
		result.ManualReview = append(result.ManualReview, ReviewItem{
			File: filename, Line: lineNum, Reason: "control flow block requires manual conversion", Original: strings.TrimSpace(line),
		})
		return line
	}

	// Convert simple expressions inline
	return reSimpleExpr.ReplaceAllStringFunc(line, func(match string) string {
		return convertExpression(match)
	})
}

func convertExpression(expr string) string {
	// Strip {{ }} and whitespace/dashes
	inner := expr
	inner = strings.TrimPrefix(inner, "{{-")
	inner = strings.TrimPrefix(inner, "{{")
	inner = strings.TrimSuffix(inner, "-}}")
	inner = strings.TrimSuffix(inner, "}}")
	inner = strings.TrimSpace(inner)

	// Handle piped expressions
	parts := splitPipe(inner)
	if 0 == len(parts) {
		return expr
	}

	base := convertBaseRef(parts[0])
	if "" == base {
		return expr // cannot convert, leave as-is
	}

	if 1 == len(parts) {
		return "${" + base + "}"
	}

	filters := make([]string, 0, len(parts)-1)
	for _, p := range parts[1:] {
		f := convertFilter(strings.TrimSpace(p))
		filters = append(filters, f)
	}
	return "${" + base + " | " + strings.Join(filters, " | ") + "}"
}

func convertBaseRef(ref string) string {
	ref = strings.TrimSpace(ref)

	replacements := map[string]string{
		".Release.Name":      "release.name",
		".Release.Namespace": "release.namespace",
		".Chart.Name":        "package.name",
		".Chart.Version":     "package.version",
		".Chart.AppVersion":  "package.appVersion",
	}

	if mapped, ok := replacements[ref]; ok {
		return mapped
	}

	if strings.HasPrefix(ref, ".Values.") {
		return "values." + ref[len(".Values."):]
	}

	return ""
}

func convertFilter(filter string) string {
	filter = strings.TrimSpace(filter)

	// default "value"
	if arg, ok := strings.CutPrefix(filter, "default "); ok {
		return "default(" + strings.TrimSpace(arg) + ")"
	}

	// indent N
	if arg, ok := strings.CutPrefix(filter, "indent "); ok {
		return "indent(" + strings.TrimSpace(arg) + ")"
	}

	// nindent N
	if arg, ok := strings.CutPrefix(filter, "nindent "); ok {
		return "nindent(" + strings.TrimSpace(arg) + ")"
	}

	// Simple renames
	filterMap := map[string]string{
		"quote":  "quote",
		"upper":  "upper",
		"lower":  "lower",
		"b64enc": "b64encode",
		"b64dec": "b64decode",
		"toYaml": "toYaml",
		"toJson": "toJson",
		"trim":   "trim",
	}

	if mapped, ok := filterMap[filter]; ok {
		return mapped
	}

	return filter
}

func splitPipe(s string) []string {
	// Split on | but not inside quotes
	parts := make([]string, 0, 4)
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)
	sLen := len(s)

	for i := range sLen {
		ch := s[i]
		if inQuote {
			current.WriteByte(ch)
			if ch == quoteChar {
				inQuote = false
			}
			continue
		}
		if '"' == ch || '\'' == ch {
			inQuote = true
			quoteChar = ch
			current.WriteByte(ch)
			continue
		}
		if '|' == ch {
			parts = append(parts, strings.TrimSpace(current.String()))
			current.Reset()
			continue
		}
		current.WriteByte(ch)
	}
	if current.Len() > 0 {
		parts = append(parts, strings.TrimSpace(current.String()))
	}
	return parts
}

func extractIndent(line string) string {
	lineLen := len(line)
	for i := range lineLen {
		if ' ' != line[i] && '\t' != line[i] {
			return line[:i]
		}
	}
	return line
}

func convertHelpers(srcPath, outputDir string, result *MigrateResult) error {
	data, err := os.ReadFile(srcPath)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrInternal, "cannot read _helpers.tpl", err)
	}

	blocks, dupes := parseDefineBlocksWithWarnings(string(data))
	for _, name := range dupes {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("_helpers.tpl: duplicate {{ define %q }} — only the last definition is kept", name))
	}
	helpers := make(map[string]string, len(blocks))
	for name, body := range blocks {
		converted := convertTemplateContent(body, "_helpers.tpl", result)
		helpers[name] = strings.TrimSpace(converted)
	}

	outData, err := yaml.Marshal(helpers)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrInternal, "failed to marshal _helpers.yaml", err)
	}

	outTemplates := filepath.Join(outputDir, "templates")
	if mkErr := os.MkdirAll(outTemplates, 0o755); nil != mkErr {
		return keramoserr.WrapError(keramoserr.ErrInternal, "cannot create templates dir", mkErr)
	}

	dest := filepath.Join(outTemplates, "_helpers.yaml")
	if err := os.WriteFile(dest, outData, 0o644); nil != err {
		return keramoserr.WrapError(keramoserr.ErrInternal, "failed to write _helpers.yaml", err)
	}
	result.ConvertedFiles = append(result.ConvertedFiles, filepath.Join("templates", "_helpers.yaml"))
	return nil
}

var reDefine = regexp.MustCompile(`\{\{-?\s*define\s+"([^"]+)"\s*-?\}\}`)
var reEndBlock = regexp.MustCompile(`\{\{-?\s*end\s*-?\}\}`)
var reBlockOpener = regexp.MustCompile(`\{\{-?\s*(if|range|with|block)\b`)

// parseDefineBlocksWithWarnings is the variant that also returns a list of
// names that were declared more than once (later definitions silently
// overwrite earlier ones, which can hide bugs in real charts).
func parseDefineBlocksWithWarnings(content string) (map[string]string, []string) {
	blocks := make(map[string]string)
	var dupes []string
	collect := func(name string) {
		if _, exists := blocks[name]; exists {
			dupes = append(dupes, name)
		}
	}
	_ = collect
	out := parseDefineBlocksImpl(content, func(name string) {
		if _, exists := blocks[name]; exists {
			dupes = append(dupes, name)
		}
	}, blocks)
	return out, dupes
}

func parseDefineBlocks(content string) map[string]string {
	out, _ := parseDefineBlocksWithWarnings(content)
	return out
}

func parseDefineBlocksImpl(content string, onDuplicate func(string), blocks map[string]string) map[string]string {
	if nil == blocks {
		blocks = make(map[string]string)
	}
	lines := strings.Split(content, "\n")
	lineCount := len(lines)

	var currentName string
	var body strings.Builder
	nesting := 0

	for i := range lineCount {
		line := lines[i]

		if m := reDefine.FindStringSubmatch(line); "" == currentName && nil != m {
			currentName = m[1]
			if nil != onDuplicate {
				if _, exists := blocks[currentName]; exists {
					onDuplicate(currentName)
				}
			}
			body.Reset()
			nesting = 0
			continue
		}

		if "" != currentName {
			openers := len(reBlockOpener.FindAllStringIndex(line, -1))
			closers := len(reEndBlock.FindAllStringIndex(line, -1))

			// Account for inner openers/closers before deciding whether this
			// line is the define's terminator.
			if 0 == nesting && 0 < closers && openers < closers {
				// The first {{ end }} on this line closes the define.
				blocks[currentName] = body.String()
				currentName = ""
				nesting = 0
				continue
			}

			nesting += openers - closers
			if nesting < 0 {
				nesting = 0
			}

			if body.Len() > 0 {
				body.WriteString("\n")
			}
			body.WriteString(line)
		}
	}
	return blocks
}

func convertNotes(notesPath, outputDir string, result *MigrateResult) error {
	data, err := os.ReadFile(notesPath)
	if nil != err {
		return nil // NOTES.txt is optional
	}

	converted := convertTemplateContent(string(data), "NOTES.txt", result)
	// Wrap in notes.yaml format: message: | block scalar
	var sb strings.Builder
	sb.WriteString("message: |\n")
	for line := range strings.SplitSeq(strings.TrimRight(converted, "\n"), "\n") {
		sb.WriteString("  ")
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
	yamlContent := sb.String()

	outTemplates := filepath.Join(outputDir, "templates")
	if mkErr := os.MkdirAll(outTemplates, 0o755); nil != mkErr {
		return keramoserr.WrapError(keramoserr.ErrInternal, "cannot create templates directory", mkErr)
	}

	dest := filepath.Join(outTemplates, "notes.yaml")
	if writeErr := os.WriteFile(dest, []byte(yamlContent), 0o644); nil != writeErr {
		return keramoserr.WrapError(keramoserr.ErrInternal, "failed to write notes.yaml", writeErr)
	}
	result.ConvertedFiles = append(result.ConvertedFiles, filepath.Join("templates", "notes.yaml"))
	return nil
}

func generateKeramosIgnore(outputDir string, result *MigrateResult) error {
	content := "# keramos ignore file\n*.bak\n*.swp\n.git/\n"
	dest := filepath.Join(outputDir, ".keramosignore")
	if err := os.WriteFile(dest, []byte(content), 0o644); nil != err {
		return keramoserr.WrapError(keramoserr.ErrInternal, "failed to write .keramosignore", err)
	}
	result.ConvertedFiles = append(result.ConvertedFiles, ".keramosignore")
	return nil
}

func copyFileIfExists(src, dst string, result *MigrateResult) error {
	data, err := os.ReadFile(src)
	if nil != err {
		return nil // optional file
	}
	if err := os.WriteFile(dst, data, 0o644); nil != err {
		return keramoserr.WrapErrorf(keramoserr.ErrInternal, err, "cannot write %s", dst)
	}
	result.ConvertedFiles = append(result.ConvertedFiles, filepath.Base(dst))
	return nil
}

// ConvertHelmExpression converts a single Helm template expression to keramos syntax.
// Exported for testing.
func ConvertHelmExpression(expr string) string {
	return convertExpression(expr)
}

// ConvertHelmLine converts a single line of Helm template content.
// Exported for testing.
func ConvertHelmLine(line, filename string, lineNum int, result *MigrateResult) string {
	return convertLine(line, filename, lineNum, result)
}

// ParseHelmChart parses a Chart.yaml file and returns the keramos.yaml content.
// Exported for testing.
func ParseHelmChart(chartYAML []byte) ([]byte, error) {
	var chart helmChart
	if err := yaml.Unmarshal(chartYAML, &chart); nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrParse, "invalid Chart.yaml", err)
	}

	meta := keramosMeta{
		APIVersion:  "keramos/v1",
		Name:        chart.Name,
		Version:     chart.Version,
		AppVersion:  chart.AppVersion,
		Description: chart.Description,
		Type:        chart.Type,
		KubeVersion: chart.KubeVersion,
		Keywords:    chart.Keywords,
		Annotations: chart.Annotations,
	}

	if len(chart.Dependencies) > 0 {
		deps := make([]keramosDep, 0, len(chart.Dependencies))
		for _, d := range chart.Dependencies {
			deps = append(deps, keramosDep(d))
		}
		meta.Dependencies = deps
	}

	if len(chart.Maintainers) > 0 {
		maintainers := make([]keramosMaintainer, 0, len(chart.Maintainers))
		for _, m := range chart.Maintainers {
			maintainers = append(maintainers, keramosMaintainer{
				Name:  m["name"],
				Email: m["email"],
			})
		}
		meta.Maintainers = maintainers
	}

	return yaml.Marshal(meta)
}

// ParseDefineBlocks parses Go template define blocks from _helpers.tpl content.
// Exported for testing.
func ParseDefineBlocks(content string) map[string]string {
	return parseDefineBlocks(content)
}

