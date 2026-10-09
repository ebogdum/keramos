package migrate

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
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
	APIVersion   string              `yaml:"apiVersion"`
	Name         string              `yaml:"name"`
	Version      string              `yaml:"version"`
	AppVersion   string              `yaml:"appVersion,omitempty"`
	Description  string              `yaml:"description,omitempty"`
	Type         string              `yaml:"type,omitempty"`
	KubeVersion  string              `yaml:"kubeVersion,omitempty"`
	Dependencies []helmDep           `yaml:"dependencies,omitempty"`
	Maintainers  []map[string]string `yaml:"maintainers,omitempty"`
	Keywords     []string            `yaml:"keywords,omitempty"`
	Annotations  map[string]string   `yaml:"annotations,omitempty"`
	Sources      []string            `yaml:"sources,omitempty"`
	Home         string              `yaml:"home,omitempty"`
	Icon         string              `yaml:"icon,omitempty"`
	Deprecated   bool                `yaml:"deprecated,omitempty"`
	Extra        map[string]any      `yaml:",inline"`
	RawEntries   []map[string]any    `yaml:"-"`
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
	APIVersion   string              `yaml:"apiVersion"`
	Name         string              `yaml:"name"`
	Version      string              `yaml:"version"`
	AppVersion   string              `yaml:"appVersion,omitempty"`
	Description  string              `yaml:"description,omitempty"`
	Type         string              `yaml:"type,omitempty"`
	KubeVersion  string              `yaml:"kubeVersion,omitempty"`
	Layers       []keramosLayer      `yaml:"layers,omitempty"`
	Dependencies []keramosDep        `yaml:"dependencies,omitempty"`
	Maintainers  []keramosMaintainer `yaml:"maintainers,omitempty"`
	Keywords     []string            `yaml:"keywords,omitempty"`
	Annotations  map[string]string   `yaml:"annotations,omitempty"`
}

type keramosLayer struct {
	Name      string   `yaml:"name"`
	Source    string   `yaml:"source"`
	Condition string   `yaml:"condition,omitempty"`
	Tags      []string `yaml:"tags,omitempty"`
	Alias     string   `yaml:"alias,omitempty"`
	Enabled   *bool    `yaml:"enabled,omitempty"`
}

func splitDependencies(deps []helmDep, subcharts []string) ([]keramosLayer, []keramosDep) {
	local := make(map[string]bool, len(subcharts))
	for _, name := range subcharts {
		local[name] = true
	}
	layers := make([]keramosLayer, 0, len(deps))
	remote := make([]keramosDep, 0, len(deps))
	seen := make(map[string]bool, len(deps))
	for _, d := range deps {
		name := d.Name
		if "" != d.Alias {
			name = d.Alias
		}
		if !local[d.Name] {
			remote = append(remote, keramosDep(d))
			continue
		}
		seen[d.Name] = true
		layers = append(layers, keramosLayer{
			Name:      name,
			Source:    "charts/" + d.Name,
			Condition: d.Condition,
			Tags:      d.Tags,
			Enabled:   d.Enabled,
		})
	}
	for _, name := range subcharts {
		if seen[name] {
			continue
		}
		layers = append(layers, keramosLayer{Name: name, Source: "charts/" + name})
	}
	if 0 == len(layers) {
		layers = nil
	}
	if 0 == len(remote) {
		remote = nil
	}
	return layers, remote
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

	writeErr := migrateWriteAll(chartPath, &chart, tmpDir, "", result)
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
func migrateWriteAll(chartPath string, chart *helmChart, writeDir, valuePrefix string, result *MigrateResult) error {
	if mkErr := os.MkdirAll(writeDir, 0o755); nil != mkErr {
		return keramoserr.WrapError(keramoserr.ErrInternal, "cannot create output directory", mkErr)
	}

	subcharts, err := localSubcharts(chartPath)
	if nil != err {
		return err
	}

	if err := convertChartYAML(chart, writeDir, subcharts, result); nil != err {
		return err
	}

	if err := writeValues(chartPath, writeDir, valuePrefix, result); nil != err {
		return err
	}
	if err := copyFileIfExists(filepath.Join(chartPath, "values.schema.json"), filepath.Join(writeDir, "values.schema.json"), result); nil != err {
		return err
	}

	conv := newTemplateConverter(result)
	conv.chartName = chart.Name
	conv.valuePrefix = valuePrefix

	templatesDir := filepath.Join(chartPath, "templates")
	if _, statErr := os.Stat(templatesDir); nil == statErr {
		if err := convertTemplates(templatesDir, writeDir, conv, result); nil != err {
			return err
		}
	}

	notesPath := filepath.Join(chartPath, "templates", "NOTES.txt")
	if err := convertNotes(notesPath, writeDir, conv, result); nil != err {
		return err
	}

	if err := writeHelpers(conv, writeDir, result); nil != err {
		return err
	}

	if err := copyChartFiles(chartPath, writeDir, result); nil != err {
		return err
	}

	// Copy crds/ verbatim (helm convention preserved by keramos's --include-crds).
	if err := copyCRDsDir(chartPath, writeDir, result); nil != err {
		return err
	}

	if err := generateKeramosIgnore(writeDir, result); nil != err {
		return err
	}

	return convertSubcharts(chartPath, writeDir, subcharts, result)
}

func localSubcharts(chartPath string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(chartPath, "charts"))
	if nil != err {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, keramoserr.WrapError(keramoserr.ErrInternal, "cannot read charts directory", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, statErr := os.Stat(filepath.Join(chartPath, "charts", entry.Name(), "Chart.yaml")); nil != statErr {
			continue
		}
		names = append(names, entry.Name())
	}
	return names, nil
}

func convertSubcharts(chartPath, writeDir string, subcharts []string, result *MigrateResult) error {
	for _, name := range subcharts {
		src := filepath.Join(chartPath, "charts", name)
		data, readErr := os.ReadFile(filepath.Join(src, "Chart.yaml"))
		if nil != readErr {
			return keramoserr.WrapErrorf(keramoserr.ErrInternal, readErr, "cannot read Chart.yaml of subchart %s", name)
		}
		var sub helmChart
		if err := yaml.Unmarshal(data, &sub); nil != err {
			return keramoserr.WrapErrorf(keramoserr.ErrParse, err, "invalid Chart.yaml in subchart %s", name)
		}
		dest := filepath.Join(writeDir, "charts", name)
		if err := migrateWriteAll(src, &sub, dest, name, result); nil != err {
			return err
		}
	}
	return nil
}

func writeValues(chartPath, writeDir, valuePrefix string, result *MigrateResult) error {
	data, err := os.ReadFile(filepath.Join(chartPath, "values.yaml"))
	if nil != err {
		return nil
	}
	if "" == valuePrefix {
		if writeErr := os.WriteFile(filepath.Join(writeDir, "values.yaml"), data, 0o644); nil != writeErr {
			return keramoserr.WrapError(keramoserr.ErrInternal, "cannot write values.yaml", writeErr)
		}
		result.ConvertedFiles = append(result.ConvertedFiles, "values.yaml")
		return nil
	}
	var values map[string]any
	if err := yaml.Unmarshal(data, &values); nil != err {
		return keramoserr.WrapErrorf(keramoserr.ErrParse, err, "invalid values.yaml in subchart %s", valuePrefix)
	}
	nested, marshalErr := yaml.Marshal(map[string]any{valuePrefix: values})
	if nil != marshalErr {
		return keramoserr.WrapError(keramoserr.ErrInternal, "cannot nest subchart values", marshalErr)
	}
	if writeErr := os.WriteFile(filepath.Join(writeDir, "values.yaml"), nested, 0o644); nil != writeErr {
		return keramoserr.WrapError(keramoserr.ErrInternal, "cannot write values.yaml", writeErr)
	}
	result.ConvertedFiles = append(result.ConvertedFiles, filepath.Join("charts", valuePrefix, "values.yaml"))
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
	return dryRunConvertTemplateTree(templatesDir, "", newTemplateConverter(result), result)
}

func dryRunConvertTemplateTree(dir, rel string, conv *templateConverter, result *MigrateResult) error {
	entries, err := os.ReadDir(dir)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrInternal, "cannot read templates directory", err)
	}

	helpersSeen := false
	for _, entry := range entries {
		name := entry.Name()
		srcPath := filepath.Join(dir, name)
		relPath := filepath.Join(rel, name)

		if entry.IsDir() {
			if "tests" == name && "" == rel {
				if testErr := dryRunConvertTestTemplates(srcPath, conv, result); nil != testErr {
					return testErr
				}
				continue
			}
			if treeErr := dryRunConvertTemplateTree(srcPath, relPath, conv, result); nil != treeErr {
				return treeErr
			}
			continue
		}

		if "NOTES.txt" == name {
			result.ConvertedFiles = append(result.ConvertedFiles, filepath.Join("templates", "notes.yaml"))
			continue
		}

		if isPartialTemplate(name) {
			if !helpersSeen {
				result.ConvertedFiles = append(result.ConvertedFiles, filepath.Join("templates", "_helpers.yaml"))
				helpersSeen = true
			}
			data, readErr := os.ReadFile(srcPath)
			if nil != readErr {
				return keramoserr.WrapErrorf(keramoserr.ErrInternal, readErr, "cannot read template %s", relPath)
			}
			conv.convertFile(string(data), relPath)
			continue
		}

		data, readErr := os.ReadFile(srcPath)
		if nil != readErr {
			return keramoserr.WrapErrorf(keramoserr.ErrInternal, readErr, "cannot read template %s", relPath)
		}

		content := string(data)
		if isHookTemplate(content) {
			result.ConvertedFiles = append(result.ConvertedFiles, filepath.Join("hooks", hookName(relPath)))
			stripped := stripHookAnnotation(content)
			conv.convertFile(stripped, relPath)
			continue
		}

		conv.convertFile(content, relPath)
		result.ConvertedFiles = append(result.ConvertedFiles, filepath.Join("templates", relPath))
	}

	return nil
}

// dryRunConvertTestTemplates reads and converts test templates in memory without writing.
func dryRunConvertTestTemplates(testsDir string, conv *templateConverter, result *MigrateResult) error {
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
		conv.convertFile(content, filepath.Join("tests", entry.Name()))
		result.ConvertedFiles = append(result.ConvertedFiles, filepath.Join("tests", entry.Name()))
	}

	return nil
}

func convertChartYAML(chart *helmChart, outputDir string, subcharts []string, result *MigrateResult) error {
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

	meta.Layers, meta.Dependencies = splitDependencies(chart.Dependencies, subcharts)

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

func convertTemplates(templatesDir, outputDir string, conv *templateConverter, result *MigrateResult) error {
	outTemplates := filepath.Join(outputDir, "templates")
	if err := os.MkdirAll(outTemplates, 0o755); nil != err {
		return keramoserr.WrapError(keramoserr.ErrInternal, "cannot create templates directory", err)
	}
	return convertTemplateTree(templatesDir, "", outputDir, conv, result)
}

var skipCopyDirs = map[string]bool{"templates": true, "charts": true, "crds": true, ".git": true}

func copyChartFiles(chartPath, outputDir string, result *MigrateResult) error {
	entries, err := os.ReadDir(chartPath)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrInternal, "cannot read chart directory", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if skipCopyDirs[name] || "Chart.yaml" == name || "values.yaml" == name ||
			"values.schema.json" == name || strings.HasPrefix(name, ".") {
			continue
		}
		src := filepath.Join(chartPath, name)
		dst := filepath.Join(outputDir, "files", name)
		if err := copyTree(src, dst, result); nil != err {
			return err
		}
	}
	return nil
}

func copyTree(src, dst string, result *MigrateResult) error {
	info, err := os.Lstat(src)
	if nil != err {
		return keramoserr.WrapErrorf(keramoserr.ErrInternal, err, "cannot stat %s", src)
	}
	if 0 != info.Mode()&os.ModeSymlink {
		return nil
	}
	if !info.IsDir() {
		data, readErr := os.ReadFile(src)
		if nil != readErr {
			return keramoserr.WrapErrorf(keramoserr.ErrInternal, readErr, "cannot read %s", src)
		}
		if mkErr := os.MkdirAll(filepath.Dir(dst), 0o755); nil != mkErr {
			return keramoserr.WrapErrorf(keramoserr.ErrInternal, mkErr, "cannot create %s", filepath.Dir(dst))
		}
		if writeErr := os.WriteFile(dst, data, 0o644); nil != writeErr {
			return keramoserr.WrapErrorf(keramoserr.ErrInternal, writeErr, "cannot write %s", dst)
		}
		return nil
	}
	entries, readErr := os.ReadDir(src)
	if nil != readErr {
		return keramoserr.WrapErrorf(keramoserr.ErrInternal, readErr, "cannot read %s", src)
	}
	for _, entry := range entries {
		if err := copyTree(filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name()), result); nil != err {
			return err
		}
	}
	return nil
}

func writeHelpers(conv *templateConverter, outputDir string, result *MigrateResult) error {
	data, err := conv.helpersDocument()
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrInternal, "failed to marshal _helpers.yaml", err)
	}
	if 0 == len(data) {
		return nil
	}
	outTemplates := filepath.Join(outputDir, "templates")
	if mkErr := os.MkdirAll(outTemplates, 0o755); nil != mkErr {
		return keramoserr.WrapError(keramoserr.ErrInternal, "cannot create templates dir", mkErr)
	}
	dest := filepath.Join(outTemplates, "_helpers.yaml")
	if writeErr := os.WriteFile(dest, data, 0o644); nil != writeErr {
		return keramoserr.WrapError(keramoserr.ErrInternal, "failed to write _helpers.yaml", writeErr)
	}
	result.ConvertedFiles = append(result.ConvertedFiles, filepath.Join("templates", "_helpers.yaml"))
	return nil
}

func isPartialTemplate(name string) bool {
	return strings.HasPrefix(name, "_")
}

func hookName(relPath string) string {
	return strings.ReplaceAll(relPath, string(filepath.Separator), "-")
}

func convertTemplateTree(dir, rel, outputDir string, conv *templateConverter, result *MigrateResult) error {
	entries, err := os.ReadDir(dir)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrInternal, "cannot read templates directory", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		srcPath := filepath.Join(dir, name)
		relPath := filepath.Join(rel, name)

		if entry.IsDir() {
			if "tests" == name && "" == rel {
				if err := convertTestTemplates(srcPath, outputDir, conv, result); nil != err {
					return err
				}
				continue
			}
			if err := convertTemplateTree(srcPath, relPath, outputDir, conv, result); nil != err {
				return err
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
			return keramoserr.WrapErrorf(keramoserr.ErrInternal, lerr, "cannot lstat template %s", relPath)
		}
		if 0 != lstat.Mode()&os.ModeSymlink {
			return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
				"refusing to follow symlink in templates: %s", srcPath)
		}

		data, readErr := os.ReadFile(srcPath)
		if nil != readErr {
			return keramoserr.WrapErrorf(keramoserr.ErrInternal, readErr, "cannot read template %s", relPath)
		}

		content := string(data)

		if isPartialTemplate(name) {
			conv.convertFile(content, relPath)
			continue
		}

		if isHookTemplate(content) {
			if err := convertHookTemplate(hookName(relPath), content, outputDir, conv, result); nil != err {
				return err
			}
			continue
		}

		converted := conv.convertFile(content, relPath)
		if "" == strings.TrimSpace(converted) {
			continue
		}
		destPath := filepath.Join(outputDir, "templates", relPath)
		if mkErr := os.MkdirAll(filepath.Dir(destPath), 0o755); nil != mkErr {
			return keramoserr.WrapErrorf(keramoserr.ErrInternal, mkErr, "cannot create directory for template %s", relPath)
		}
		if err := os.WriteFile(destPath, []byte(converted), 0o644); nil != err {
			return keramoserr.WrapErrorf(keramoserr.ErrInternal, err, "cannot write template %s", relPath)
		}
		result.ConvertedFiles = append(result.ConvertedFiles, filepath.Join("templates", relPath))
	}
	return nil
}

func convertTestTemplates(testsDir, outputDir string, conv *templateConverter, result *MigrateResult) error {
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
		converted := conv.convertFile(content, filepath.Join("tests", entry.Name()))
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

func convertHookTemplate(name, content, outputDir string, conv *templateConverter, result *MigrateResult) error {
	hooksDir := filepath.Join(outputDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); nil != err {
		return keramoserr.WrapError(keramoserr.ErrInternal, "cannot create hooks directory", err)
	}

	stripped := stripHookAnnotation(content)
	converted := conv.convertFile(stripped, name)
	destPath := filepath.Join(hooksDir, name)
	if err := os.WriteFile(destPath, []byte(converted), 0o644); nil != err {
		return keramoserr.WrapErrorf(keramoserr.ErrInternal, err, "cannot write hook template %s", name)
	}
	result.ConvertedFiles = append(result.ConvertedFiles, filepath.Join("hooks", name))
	return nil
}

func convertNotes(notesPath, outputDir string, conv *templateConverter, result *MigrateResult) error {
	data, err := os.ReadFile(notesPath)
	if nil != err {
		return nil // NOTES.txt is optional
	}

	converted, structured := conv.convertText(string(data), "NOTES.txt")
	if "" == strings.TrimSpace(converted) {
		return nil
	}
	yamlContent := converted
	if !structured {
		var sb strings.Builder
		sb.WriteString("message: |\n")
		for line := range strings.SplitSeq(strings.TrimRight(converted, "\n"), "\n") {
			sb.WriteString("  ")
			sb.WriteString(line)
			sb.WriteByte('\n')
		}
		yamlContent = sb.String()
	}

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
