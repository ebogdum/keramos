package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// FormatTable renders tabular data with aligned columns.
func FormatTable(headers []string, rows [][]string) string {
	colCount := len(headers)
	if 0 == colCount {
		return ""
	}

	widths := make([]int, colCount)
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i := 0; i < colCount && i < len(row); i++ {
			cellLen := len(row[i])
			if cellLen > widths[i] {
				widths[i] = cellLen
			}
		}
	}

	var b strings.Builder

	// Header row
	for i, h := range headers {
		if i > 0 {
			b.WriteString("    ")
		}
		b.WriteString(padRight(h, widths[i]))
	}
	b.WriteString("\n")

	// Data rows
	for _, row := range rows {
		for i := 0; i < colCount; i++ {
			if i > 0 {
				b.WriteString("    ")
			}
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			b.WriteString(padRight(cell, widths[i]))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// FormatJSON marshals data to indented JSON.
func FormatJSON(data any) (string, error) {
	out, err := json.MarshalIndent(data, "", "  ")
	if nil != err {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(out) + "\n", nil
}

// FormatYAML marshals data to YAML.
func FormatYAML(data any) (string, error) {
	out, err := yaml.Marshal(data)
	if nil != err {
		return "", fmt.Errorf("failed to marshal YAML: %w", err)
	}
	return string(out), nil
}

// validateDryRunFlag checks that the --dry-run value is valid.
func validateDryRunFlag(value string) error {
	if "" == value || "client" == value || "server" == value {
		return nil
	}
	return fmt.Errorf("invalid --dry-run value %q: must be 'client' or 'server'", value)
}

// validateOutputFlag checks that the --output value is valid.
func validateOutputFlag(value string) error {
	if "table" == value || "json" == value || "yaml" == value {
		return nil
	}
	return fmt.Errorf("invalid --output value %q: must be 'table', 'json', or 'yaml'", value)
}

// outputRelease prints a release in the requested format. When format is "table",
// the provided tableFn is called for default text output.
func outputRelease(w io.Writer, rel any, format string, tableFn func()) error {
	switch format {
	case "json":
		out, err := FormatJSON(rel)
		if nil != err {
			return err
		}
		fmt.Fprint(w, out)
		return nil
	case "yaml":
		out, err := FormatYAML(rel)
		if nil != err {
			return err
		}
		fmt.Fprint(w, out)
		return nil
	default:
		tableFn()
		return nil
	}
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
