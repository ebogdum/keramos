package layer

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ebogdum/keramos/v3/internal/deptree"
	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
	"github.com/ebogdum/keramos/v3/internal/pkg"
	"gopkg.in/yaml.v3"
)

// ResolvedPackage represents a fully resolved package with all layers merged.
type ResolvedPackage struct {
	Metadata  *pkg.PackageMetadata
	Values    pkg.Values
	Templates map[string]string
	Partials  map[string]any
	Hooks     map[string]string
	Tests     map[string]string
	Files     map[string][]byte
}

// Resolve loads a package from disk, resolving all layers and optionally a profile.
// Uses the dependency tree for proper merge ordering.
func Resolve(packagePath string, profile string) (*ResolvedPackage, error) {
	return ResolveWithOverrides(packagePath, profile, nil)
}

// ResolveWithOverrides is the variant that threads user-supplied value
// overrides into condition/tags evaluation when filtering enabled layers.
func ResolveWithOverrides(packagePath string, profile string, overrides map[string]any) (*ResolvedPackage, error) {
	absPath, err := filepath.Abs(packagePath)
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrPackageInvalid, "failed to resolve package path", err)
	}

	// Phase 1: Build tree structure (with override-aware sub-chart filtering)
	root, buildErr := deptree.BuildWithOverrides(absPath, overrides)
	if nil != buildErr {
		return nil, buildErr
	}

	// Phase 2: Load all content
	if popErr := deptree.Populate(root); nil != popErr {
		return nil, popErr
	}

	// Phase 3: Merge using tree
	mergedValues, mergeErr := deptree.MergeValues(root)
	if nil != mergeErr {
		return nil, mergeErr
	}

	templates, partials, tmplErr := deptree.MergeTemplates(root)
	if nil != tmplErr {
		return nil, tmplErr
	}

	hooks := deptree.MergeHooks(root)
	tests := deptree.MergeTests(root)

	files, filesErr := loadPackageFiles(absPath)
	if nil != filesErr {
		return nil, filesErr
	}

	resolved := &ResolvedPackage{
		Metadata:  root.Metadata,
		Values:    pkg.Values(mergedValues),
		Templates: templates,
		Partials:  partials,
		Hooks:     hooks,
		Tests:     tests,
		Files:     files,
	}

	if "" != profile {
		profileErr := applyProfile(absPath, profile, resolved)
		if nil != profileErr {
			return nil, profileErr
		}
	}

	return resolved, nil
}

// BuildTree builds and returns the dependency tree for a package path.
// Exposed for use by CLI commands (e.g. keramos dep tree).
func BuildTree(packagePath string) (*deptree.Node, error) {
	absPath, err := filepath.Abs(packagePath)
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrPackageInvalid, "failed to resolve package path", err)
	}
	return deptree.Build(absPath)
}

func loadValuesOptional(dirPath string) (pkg.Values, error) {
	vals, err := pkg.LoadValues(dirPath)
	if nil != err {
		if os.IsNotExist(extractCause(err)) {
			return make(pkg.Values), nil
		}
		return nil, err
	}
	if nil == vals {
		return make(pkg.Values), nil
	}
	return pkg.Values(normalizeMap(map[string]any(vals))), nil
}

// normalizeMap recursively converts any named map types (like pkg.Values)
// to plain map[string]any so that type assertions work uniformly.
func normalizeMap(m map[string]any) map[string]any {
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = normalizeValue(v)
	}
	return result
}

func normalizeValue(v any) any {
	if nil == v {
		return nil
	}
	if m, ok := v.(pkg.Values); ok {
		return normalizeMap(map[string]any(m))
	}
	if m, ok := v.(map[string]any); ok {
		return normalizeMap(m)
	}
	if s, ok := v.([]any); ok {
		result := make([]any, len(s))
		for i, item := range s {
			result[i] = normalizeValue(item)
		}
		return result
	}
	return v
}

func extractCause(err error) error {
	he, ok := err.(*keramoserr.KeramosError)
	if ok && nil != he.Cause {
		return he.Cause
	}
	return err
}

// loadPackageFiles walks the package directory and returns a `Files` map
// (relative path → raw bytes) for every file outside the directories the
// engine handles specially (templates/, hooks/, tests/, crds/, charts/,
// profiles/, policies/, .git, .keramosignore). The result is the input
// templates see as `.Files`, supporting `Files.Get`, `Files.GetBytes`,
// `Files.Glob`, etc.
func loadPackageFiles(packagePath string) (map[string][]byte, error) {
	files := make(map[string][]byte)
	skipDirs := map[string]bool{
		"templates": true, "hooks": true, "tests": true,
		"crds": true, "charts": true, "profiles": true,
		"policies": true, ".git": true,
	}
	walkErr := filepath.Walk(packagePath, func(path string, info os.FileInfo, e error) error {
		if nil != e {
			return e
		}
		if info.IsDir() {
			rel, _ := filepath.Rel(packagePath, path)
			if "" == rel || "." == rel {
				return nil
			}
			top := strings.SplitN(rel, string(filepath.Separator), 2)[0]
			if skipDirs[top] {
				return filepath.SkipDir
			}
			return nil
		}
		// Skip keramos.yaml and values.yaml — they're metadata, not user files.
		base := filepath.Base(path)
		if "keramos.yaml" == base || "values.yaml" == base ||
			"values.schema.json" == base || ".keramosignore" == base {
			return nil
		}
		rel, relErr := filepath.Rel(packagePath, path)
		if nil != relErr {
			return relErr
		}
		data, rdErr := os.ReadFile(path)
		if nil != rdErr {
			return keramoserr.WrapErrorf(keramoserr.ErrPackageInvalid, rdErr, "read file %s", path)
		}
		files[rel] = data
		return nil
	})
	if nil != walkErr {
		return nil, walkErr
	}
	return files, nil
}

func loadTemplates(dirPath string) (map[string]string, map[string]any, error) {
	templatesDir := filepath.Join(dirPath, "templates")
	templates := make(map[string]string)
	partials := make(map[string]any)

	info, err := os.Stat(templatesDir)
	if nil != err {
		if os.IsNotExist(err) {
			return templates, partials, nil
		}
		return nil, nil, keramoserr.WrapError(keramoserr.ErrPackageInvalid, "failed to stat templates directory", err)
	}
	if !info.IsDir() {
		return templates, partials, nil
	}

	entries, err := os.ReadDir(templatesDir)
	if nil != err {
		return nil, nil, keramoserr.WrapError(keramoserr.ErrPackageInvalid, "failed to read templates directory", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !isYAMLFile(name) {
			continue
		}

		data, readErr := os.ReadFile(filepath.Join(templatesDir, name))
		if nil != readErr {
			return nil, nil, keramoserr.WrapError(keramoserr.ErrPackageInvalid, "failed to read template file", readErr)
		}

		if strings.HasPrefix(name, "_") {
			// Partial files are YAML maps whose top-level keys become
			// individually-addressable partials so ${include "key"} can
			// resolve them. The filename-keyed form is also kept so
			// callers that look up by filename still work.
			var parsed map[string]any
			if pErr := yaml.Unmarshal(data, &parsed); nil != pErr {
				return nil, nil, keramoserr.WrapErrorf(keramoserr.ErrPackageInvalid, pErr,
					"failed to parse partials file %s", name)
			}
			partials[name] = string(data)
			for k, v := range parsed {
				partials[k] = v
			}
		} else {
			templates[name] = string(data)
		}
	}

	return templates, partials, nil
}

func applyProfile(packagePath, profile string, resolved *ResolvedPackage) error {
	profileDir := filepath.Join(packagePath, "profiles", profile)
	info, err := os.Stat(profileDir)
	if nil != err {
		if !os.IsNotExist(err) {
			return keramoserr.WrapError(keramoserr.ErrPackageInvalid, "failed to stat profile directory", err)
		}
		// Fall back to single-file profile: profiles/<name>.yaml. This
		// matches the common convention for lightweight overrides that
		// don't need their own templates directory.
		for _, ext := range []string{".yaml", ".yml"} {
			singleFile := filepath.Join(packagePath, "profiles", profile+ext)
			if data, readErr := os.ReadFile(singleFile); nil == readErr {
				var fileVals pkg.Values
				if uErr := yaml.Unmarshal(data, &fileVals); nil != uErr {
					return keramoserr.WrapErrorf(keramoserr.ErrPackageInvalid, uErr,
						"parse profile file %s", singleFile)
				}
				if 0 < len(fileVals) {
					resolved.Values = pkg.Values(DeepMerge(map[string]any(resolved.Values), map[string]any(fileVals)))
				}
				return nil
			}
		}
		return keramoserr.NewErrorf(keramoserr.ErrPackageInvalid, "profile not found: %s", profile)
	}
	if !info.IsDir() {
		return keramoserr.NewErrorf(keramoserr.ErrPackageInvalid, "profile path is not a directory: %s", profile)
	}

	profileValues, err := loadValuesOptional(profileDir)
	if nil != err {
		return err
	}
	if 0 < len(profileValues) {
		resolved.Values = pkg.Values(DeepMerge(map[string]any(resolved.Values), map[string]any(profileValues)))
	}

	profileTemplates, profilePartials, err := loadTemplates(profileDir)
	if nil != err {
		return err
	}
	for name, content := range profileTemplates {
		resolved.Templates[name] = content
	}
	for name, content := range profilePartials {
		resolved.Partials[name] = content
	}

	return nil
}

func isYAMLFile(name string) bool {
	return strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")
}
