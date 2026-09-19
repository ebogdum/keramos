package values

import (
	"os"

	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
	"github.com/ebogdum/keramos/v3/internal/layer"
	"gopkg.in/yaml.v3"
)

// Resolve merges values from multiple sources with precedence:
// 1. Package defaults (values.yaml) -- lowest
// 2. -f/--values file overrides (multiple, left to right)
// 3. --set, --set-string, --set-file, --set-json overrides -- highest
func Resolve(defaults map[string]any, valueFiles []string, sets []string) (map[string]any, error) {
	return ResolveAll(defaults, valueFiles, sets, nil, nil, nil)
}

// ResolveAll is the full-featured resolver covering every set-style override.
func ResolveAll(defaults map[string]any, valueFiles, sets, setStrings, setFiles, setJSON []string) (map[string]any, error) {
	result := layer.DeepMerge(nil, defaults)

	for _, filePath := range valueFiles {
		fileVals, err := loadValuesFile(filePath)
		if nil != err {
			return nil, err
		}
		result = layer.DeepMerge(result, fileVals)
	}

	for _, s := range sets {
		if err := ParseSet(result, s); nil != err {
			return nil, err
		}
	}
	for _, s := range setStrings {
		if err := ParseSetString(result, s); nil != err {
			return nil, err
		}
	}
	for _, s := range setFiles {
		if err := ParseSetFile(result, s); nil != err {
			return nil, err
		}
	}
	for _, s := range setJSON {
		if err := ParseSetJSON(result, s); nil != err {
			return nil, err
		}
	}

	return result, nil
}

func loadValuesFile(filePath string) (map[string]any, error) {
	data, err := os.ReadFile(filePath)
	if nil != err {
		return nil, keramoserr.WrapErrorf(keramoserr.ErrCLIFlag, err, "failed to read values file: %s", filePath)
	}

	var vals map[string]any
	if err := yaml.Unmarshal(data, &vals); nil != err {
		return nil, keramoserr.WrapErrorf(keramoserr.ErrCLIFlag, err, "failed to parse values file: %s", filePath)
	}

	if nil == vals {
		return make(map[string]any), nil
	}
	return vals, nil
}
