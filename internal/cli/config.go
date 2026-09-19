package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// newConfigCommand interactively walks `<package>/values.schema.json`,
// prompting for each leaf with type-aware validation, then writes the
// result to a values file. This is the schema-aware values config UI.
func newConfigCommand() *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:   "config <package-path>",
		Short: "Interactively build a values file from values.schema.json",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pkg := args[0]
			schemaPath := filepath.Join(pkg, "values.schema.json")
			data, err := os.ReadFile(schemaPath)
			if nil != err {
				return keramoserr.WrapError(keramoserr.ErrCLIValidation,
					"read values.schema.json (required for keramos config)", err)
			}
			var schema map[string]any
			if jErr := json.Unmarshal(data, &schema); nil != jErr {
				return keramoserr.WrapError(keramoserr.ErrCLIValidation, "parse schema", jErr)
			}
			r := bufio.NewReader(os.Stdin)
			values := map[string]any{}
			fmt.Fprintln(cmd.OutOrStdout(), "Walking schema. Press <enter> to keep defaults.")
			if walkErr := promptObject(cmd.OutOrStdout(), r, schema, values, ""); nil != walkErr {
				return walkErr
			}
			doc, mErr := yaml.Marshal(values)
			if nil != mErr {
				return keramoserr.WrapError(keramoserr.ErrInternal, "marshal values", mErr)
			}
			if "" == out || "-" == out {
				fmt.Fprint(cmd.OutOrStdout(), string(doc))
				return nil
			}
			if wErr := os.WriteFile(out, doc, 0o644); nil != wErr {
				return keramoserr.WrapError(keramoserr.ErrInternal, "write values file", wErr)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", out)
			return nil
		},
	}
	cmd.Flags().StringVarP(&out, "out", "o", "-", "output values file path (- for stdout)")
	return cmd
}

func promptObject(w interface{}, r *bufio.Reader, schema map[string]any, dst map[string]any, prefix string) error {
	props, _ := schema["properties"].(map[string]any)
	if 0 == len(props) {
		return nil
	}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	requiredList, _ := schema["required"].([]any)
	required := make(map[string]bool, len(requiredList))
	for _, k := range requiredList {
		if s, ok := k.(string); ok {
			required[s] = true
		}
	}
	for _, k := range keys {
		sub, _ := props[k].(map[string]any)
		path := k
		if "" != prefix {
			path = prefix + "." + k
		}
		t, _ := sub["type"].(string)
		switch t {
		case "object":
			child := map[string]any{}
			if err := promptObject(w, r, sub, child, path); nil != err {
				return err
			}
			if 0 < len(child) {
				dst[k] = child
			}
		default:
			val, err := promptScalar(r, path, sub, required[k])
			if nil != err {
				return err
			}
			if nil != val {
				dst[k] = val
			}
		}
	}
	return nil
}

// promptScalar reads one value from stdin, validating against the schema.
func promptScalar(r interface {
	ReadString(byte) (string, error)
}, path string, schema map[string]any, required bool) (any, error) {
	t, _ := schema["type"].(string)
	desc, _ := schema["description"].(string)
	def := schema["default"]
	prompt := fmt.Sprintf("%s (%s)", path, t)
	if "" != desc {
		prompt += " — " + desc
	}
	if nil != def {
		prompt += fmt.Sprintf(" [%v]", def)
	}
	if required {
		prompt += " *"
	}
	fmt.Fprintf(os.Stderr, "%s: ", prompt)
	line, err := r.ReadString('\n')
	if nil != err && "" == strings.TrimRight(line, "\r\n") {
		if required && nil == def {
			return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
				"%s is required but stdin closed before input was provided", path)
		}
		return def, nil
	}
	line = strings.TrimRight(line, "\r\n")
	if "" == line {
		return def, nil
	}
	switch t {
	case "integer":
		n, pErr := strconv.ParseInt(line, 10, 64)
		if nil != pErr {
			return nil, keramoserr.WrapErrorf(keramoserr.ErrCLIValidation, pErr, "%s must be an integer", path)
		}
		return n, nil
	case "number":
		f, pErr := strconv.ParseFloat(line, 64)
		if nil != pErr {
			return nil, keramoserr.WrapErrorf(keramoserr.ErrCLIValidation, pErr, "%s must be a number", path)
		}
		return f, nil
	case "boolean":
		b, pErr := strconv.ParseBool(line)
		if nil != pErr {
			return nil, keramoserr.WrapErrorf(keramoserr.ErrCLIValidation, pErr, "%s must be a boolean", path)
		}
		return b, nil
	default:
		return line, nil
	}
}
