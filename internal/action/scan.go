package action

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
	"github.com/ebogdum/keramos/v3/internal/logger"
	"github.com/ebogdum/keramos/v3/internal/pkg"
	"gopkg.in/yaml.v3"
)

// ScanResult holds the output of a scan operation.
type ScanResult struct {
	BasePackage     string
	UpdatedPackages []string
	CommonValues    map[string]any
	CommonTemplates []string
	Report          string
}

// Scan analyzes a directory of keramos packages, extracts common values and templates
// into a base layer, and rewrites packages to reference the base.
func Scan(dir string, outputDir string, dryRun bool) (*ScanResult, error) {
	absDir, err := filepath.Abs(dir)
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrPackageInvalid, "failed to resolve directory path", err)
	}

	if "" == outputDir {
		outputDir = absDir
	}
	absOutput, err := filepath.Abs(outputDir)
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrPackageInvalid, "failed to resolve output path", err)
	}

	// Step 1: Find all keramos packages
	packages, findErr := findKeramosPackages(absDir)
	if nil != findErr {
		return nil, findErr
	}

	if 2 > len(packages) {
		return nil, keramoserr.NewError(keramoserr.ErrCLIValidation, "scan requires at least 2 packages to find commonality")
	}

	// Step 2: Load values and templates from each package
	pkgs := make([]pkgData, 0, len(packages))
	for _, p := range packages {
		meta, metaErr := pkg.LoadPackageMetadata(p)
		if nil != metaErr {
			logger.Warn("skipping package at %s: %v", p, metaErr)
			continue
		}

		vals, vErr := loadScanValues(p)
		if nil != vErr {
			logger.Warn("skipping values for %s: %v", p, vErr)
			vals = make(map[string]any)
		}

		tmpls := loadScanTemplates(p)

		pkgs = append(pkgs, pkgData{
			path:      p,
			name:      meta.Name,
			values:    vals,
			templates: tmpls,
			meta:      meta,
		})
	}

	if 2 > len(pkgs) {
		return nil, keramoserr.NewError(keramoserr.ErrCLIValidation, "not enough valid packages found for scan")
	}

	// Step 3: Build frequency map for values
	threshold := len(pkgs) / 2
	if 0 == threshold {
		threshold = 1
	}

	commonValues := findCommonValues(pkgs, threshold)

	// Step 4: Find common templates
	commonTemplates := findCommonTemplates(pkgs, threshold)

	// Step 5: Generate base package
	basePath := filepath.Join(absOutput, "base")

	var report strings.Builder
	report.WriteString("Keramos Scan Report\n")
	report.WriteString(strings.Repeat("=", 40) + "\n")
	report.WriteString("\n")
	fmt.Fprintf(&report, "Packages scanned: %d\n", len(pkgs))
	fmt.Fprintf(&report, "Common values found: %d\n", countKeys(commonValues))
	fmt.Fprintf(&report, "Common templates found: %d\n", len(commonTemplates))
	report.WriteString("\n")

	if 0 == countKeys(commonValues) && 0 == len(commonTemplates) {
		report.WriteString("No common values or templates found. No base layer generated.\n")
		return &ScanResult{
			CommonValues:    commonValues,
			CommonTemplates: scanTemplateNames(commonTemplates),
			Report:          report.String(),
		}, nil
	}

	if dryRun {
		report.WriteString("[DRY RUN] Would create base layer at: " + basePath + "\n")
		report.WriteString("[DRY RUN] Would update packages:\n")
		for _, p := range pkgs {
			report.WriteString("  - " + p.name + " (" + p.path + ")\n")
		}
		return &ScanResult{
			BasePackage:     basePath,
			CommonValues:    commonValues,
			CommonTemplates: scanTemplateNames(commonTemplates),
			Report:          report.String(),
		}, nil
	}

	// Write base package
	if writeErr := writeBasePackage(basePath, commonValues, commonTemplates); nil != writeErr {
		return nil, writeErr
	}
	report.WriteString("Created base layer at: " + basePath + "\n")

	// Step 6: Rewrite packages
	updatedPaths := make([]string, 0, len(pkgs))
	for _, p := range pkgs {
		relBase, relErr := filepath.Rel(p.path, basePath)
		if nil != relErr {
			relBase = basePath
		}

		if rewriteErr := rewritePackageForBase(p.path, p.meta, p.values, commonValues, relBase); nil != rewriteErr {
			logger.Warn("failed to rewrite package %s: %v", p.name, rewriteErr)
			continue
		}
		updatedPaths = append(updatedPaths, p.path)
		report.WriteString("Updated package: " + p.name + " (" + p.path + ")\n")
	}

	return &ScanResult{
		BasePackage:     basePath,
		UpdatedPackages: updatedPaths,
		CommonValues:    commonValues,
		CommonTemplates: scanTemplateNames(commonTemplates),
		Report:          report.String(),
	}, nil
}

func findKeramosPackages(dir string) ([]string, error) {
	var packages []string

	entries, err := os.ReadDir(dir)
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrPackageInvalid, "failed to read directory", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		candidatePath := filepath.Join(dir, entry.Name())
		keramosYaml := filepath.Join(candidatePath, "keramos.yaml")
		if fileExists(keramosYaml) {
			packages = append(packages, candidatePath)
		}
	}

	return packages, nil
}

func loadScanValues(dirPath string) (map[string]any, error) {
	vals, err := pkg.LoadValues(dirPath)
	if nil != err {
		return nil, err
	}
	if nil == vals {
		return make(map[string]any), nil
	}
	return map[string]any(vals), nil
}

func loadScanTemplates(dirPath string) map[string]string {
	templatesDir := filepath.Join(dirPath, "templates")
	result := make(map[string]string)

	info, err := os.Stat(templatesDir)
	if nil != err || !info.IsDir() {
		return result
	}

	entries, err := os.ReadDir(templatesDir)
	if nil != err {
		return result
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(templatesDir, name))
		if nil != readErr {
			continue
		}
		result[name] = string(data)
	}

	return result
}

// findCommonValues finds values that appear in more than threshold packages with the same value.
func findCommonValues(pkgs []pkgData, threshold int) map[string]any {
	type valueEntry struct {
		value any
		count int
	}

	// Flatten each package's values into path -> value
	frequency := make(map[string]*valueEntry)
	totalPkgs := len(pkgs)

	for _, p := range pkgs {
		flattened := flattenValues("", p.values)
		for path, val := range flattened {
			key := path + "=" + formatValue(val)
			entry, exists := frequency[key]
			if !exists {
				frequency[key] = &valueEntry{value: val, count: 1}
				continue
			}
			entry.count++
		}
	}

	// Build common values map from entries that meet threshold
	common := make(map[string]any)
	for key, entry := range frequency {
		if entry.count <= threshold && entry.count < totalPkgs {
			continue
		}
		// Extract path from key (before "=")
		eqIdx := strings.Index(key, "=")
		if -1 == eqIdx {
			continue
		}
		path := key[:eqIdx]
		setNestedValue(common, path, entry.value)
	}

	return common
}

type pkgData struct {
	path      string
	name      string
	values    map[string]any
	templates map[string]string
	meta      pkg.PackageMetadata
}

// flattenValues converts a nested map into dot-separated path->value pairs.
func flattenValues(prefix string, m map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range m {
		path := k
		if "" != prefix {
			path = prefix + "." + k
		}

		if nested, ok := v.(map[string]any); ok {
			for nk, nv := range flattenValues(path, nested) {
				result[nk] = nv
			}
			continue
		}

		result[path] = v
	}
	return result
}

func formatValue(v any) string {
	switch val := v.(type) {
	case string:
		return "s:" + val
	case int:
		return "i:" + strconv.Itoa(val)
	case int64:
		return "i:" + strconv.FormatInt(val, 10)
	case float64:
		return "f:" + strconv.FormatFloat(val, 'g', -1, 64)
	case bool:
		if val {
			return "b:true"
		}
		return "b:false"
	default:
		data, err := yaml.Marshal(v)
		if nil != err {
			return ""
		}
		return string(data)
	}
}

func setNestedValue(m map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	current := m
	lastIdx := len(parts) - 1

	for i := 0; i < lastIdx; i++ {
		next, ok := current[parts[i]]
		if !ok {
			next = make(map[string]any)
			current[parts[i]] = next
		}
		nextMap, ok := next.(map[string]any)
		if !ok {
			nextMap = make(map[string]any)
			current[parts[i]] = nextMap
		}
		current = nextMap
	}

	current[parts[lastIdx]] = value
}

type templateMatch struct {
	name    string
	content string
}

func findCommonTemplates(pkgs []pkgData, threshold int) []templateMatch {
	// Count identical templates across packages
	type tmplEntry struct {
		content string
		count   int
	}

	frequency := make(map[string]*tmplEntry)

	for _, p := range pkgs {
		for name, content := range p.templates {
			key := name
			entry, exists := frequency[key]
			if !exists {
				frequency[key] = &tmplEntry{content: content, count: 1}
				continue
			}
			if entry.content == content {
				entry.count++
			}
		}
	}

	var result []templateMatch
	for name, entry := range frequency {
		if entry.count > threshold {
			result = append(result, templateMatch{name: name, content: entry.content})
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].name < result[j].name
	})

	return result
}

func writeBasePackage(basePath string, commonValues map[string]any, commonTemplates []templateMatch) error {
	if mkErr := os.MkdirAll(basePath, 0755); nil != mkErr {
		return keramoserr.WrapError(keramoserr.ErrPackageInvalid, "failed to create base directory", mkErr)
	}

	// Write keramos.yaml
	meta := map[string]any{
		"apiVersion": "keramos/v1",
		"name":       "base",
		"version":    "1.0.0",
	}
	metaData, err := yaml.Marshal(meta)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrPackageInvalid, "failed to marshal base keramos.yaml", err)
	}
	if writeErr := os.WriteFile(filepath.Join(basePath, "keramos.yaml"), metaData, 0644); nil != writeErr {
		return keramoserr.WrapError(keramoserr.ErrPackageInvalid, "failed to write base keramos.yaml", writeErr)
	}

	// Write values.yaml
	if 0 < len(commonValues) {
		valData, valErr := yaml.Marshal(commonValues)
		if nil != valErr {
			return keramoserr.WrapError(keramoserr.ErrPackageInvalid, "failed to marshal base values.yaml", valErr)
		}
		if writeErr := os.WriteFile(filepath.Join(basePath, "values.yaml"), valData, 0644); nil != writeErr {
			return keramoserr.WrapError(keramoserr.ErrPackageInvalid, "failed to write base values.yaml", writeErr)
		}
	}

	// Write templates
	if 0 < len(commonTemplates) {
		tmplDir := filepath.Join(basePath, "templates")
		if mkErr := os.MkdirAll(tmplDir, 0755); nil != mkErr {
			return keramoserr.WrapError(keramoserr.ErrPackageInvalid, "failed to create base templates directory", mkErr)
		}
		for _, tmpl := range commonTemplates {
			if writeErr := os.WriteFile(filepath.Join(tmplDir, tmpl.name), []byte(tmpl.content), 0644); nil != writeErr {
				return keramoserr.WrapError(keramoserr.ErrPackageInvalid, "failed to write base template", writeErr)
			}
		}
	}

	return nil
}

func rewritePackageForBase(pkgPath string, meta pkg.PackageMetadata, values map[string]any, commonValues map[string]any, relBasePath string) error {
	// Add base layer to keramos.yaml if not already present
	hasBase := false
	for _, l := range meta.Layers {
		if "base" == l.Name {
			hasBase = true
			break
		}
	}

	if !hasBase {
		meta.Layers = append([]pkg.LayerSource{{
			Name:   "base",
			Source: relBasePath,
		}}, meta.Layers...)

		metaData, err := yaml.Marshal(meta)
		if nil != err {
			return keramoserr.WrapError(keramoserr.ErrPackageInvalid, "failed to marshal updated keramos.yaml", err)
		}
		if writeErr := os.WriteFile(filepath.Join(pkgPath, "keramos.yaml"), metaData, 0644); nil != writeErr {
			return keramoserr.WrapError(keramoserr.ErrPackageInvalid, "failed to write updated keramos.yaml", writeErr)
		}
	}

	// Remove common values from the package's values.yaml
	reducedValues := removeCommonValues(values, commonValues)
	if 0 < len(reducedValues) {
		valData, err := yaml.Marshal(reducedValues)
		if nil != err {
			return keramoserr.WrapError(keramoserr.ErrPackageInvalid, "failed to marshal reduced values.yaml", err)
		}
		if writeErr := os.WriteFile(filepath.Join(pkgPath, "values.yaml"), valData, 0644); nil != writeErr {
			return keramoserr.WrapError(keramoserr.ErrPackageInvalid, "failed to write reduced values.yaml", writeErr)
		}
	} else {
		// Remove values.yaml if no values remain
		_ = os.Remove(filepath.Join(pkgPath, "values.yaml"))
	}

	return nil
}

func removeCommonValues(values, common map[string]any) map[string]any {
	if nil == values {
		return nil
	}

	result := make(map[string]any)
	for k, v := range values {
		commonVal, inCommon := common[k]
		if !inCommon {
			result[k] = v
			continue
		}

		vMap, vIsMap := v.(map[string]any)
		cMap, cIsMap := commonVal.(map[string]any)
		if vIsMap && cIsMap {
			remaining := removeCommonValues(vMap, cMap)
			if 0 < len(remaining) {
				result[k] = remaining
			}
			continue
		}

		// If the value matches the common value, skip it
		if formatValue(v) == formatValue(commonVal) {
			continue
		}
		result[k] = v
	}

	return result
}

func countKeys(m map[string]any) int {
	count := 0
	for _, v := range m {
		if nested, ok := v.(map[string]any); ok {
			count += countKeys(nested)
			continue
		}
		count++
	}
	return count
}

func scanTemplateNames(matches []templateMatch) []string {
	names := make([]string, len(matches))
	for i, t := range matches {
		names[i] = t.name
	}
	return names
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if nil != err {
		return false
	}
	return !info.IsDir()
}

