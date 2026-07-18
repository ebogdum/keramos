package cli

import (
	"fmt"
	"sort"

	"github.com/ebogdum/keramos/internal/engine"
	"github.com/ebogdum/keramos/internal/layer"
	"github.com/ebogdum/keramos/internal/logger"
	"github.com/ebogdum/keramos/internal/values"
	"github.com/spf13/cobra"
)

func newDebugCommand() *cobra.Command {
	var (
		valueFiles []string
		sets       []string
		profile    string
		trace      bool
	)

	cmd := &cobra.Command{
		Use:   "debug <package-path>",
		Short: "Debug template rendering",
		Long:  "Debug mode for template rendering with optional step-by-step trace output.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDebug(cmd, args[0], valueFiles, sets, profile, trace)
		},
	}

	cmd.Flags().StringArrayVarP(&valueFiles, "values", "f", nil, "values file overrides (repeatable)")
	cmd.Flags().StringArrayVar(&sets, "set", nil, "set key=value overrides (repeatable)")
	cmd.Flags().StringVar(&profile, "profile", "", "profile name to apply")
	cmd.Flags().BoolVar(&trace, "trace", false, "enable step-by-step rendering trace")

	return cmd
}

func runDebug(cmd *cobra.Command, packagePath string, valueFiles, sets []string, profile string, trace bool) error {
	w := cmd.OutOrStdout()

	logger.Debug("debug: resolving package at %s", packagePath)

	// Step 1: Resolve package
	resolved, err := layer.Resolve(packagePath, profile)
	if nil != err {
		return err
	}

	if trace {
		fmt.Fprintln(w, "=== PACKAGE RESOLUTION ===")
		fmt.Fprintf(w, "Package:     %s\n", resolved.Metadata.Name)
		fmt.Fprintf(w, "Version:     %s\n", resolved.Metadata.Version)
		if "" != resolved.Metadata.AppVersion {
			fmt.Fprintf(w, "App Version: %s\n", resolved.Metadata.AppVersion)
		}
		if "" != resolved.Metadata.Base {
			fmt.Fprintf(w, "Base:        %s\n", resolved.Metadata.Base)
		}
		if "" != profile {
			fmt.Fprintf(w, "Profile:     %s\n", profile)
		}
		fmt.Fprintln(w)
	}

	// Step 2: Resolve values
	mergedValues, err := values.Resolve(map[string]any(resolved.Values), valueFiles, sets)
	if nil != err {
		return err
	}

	if trace {
		fmt.Fprintln(w, "=== VALUES MERGE ===")
		fmt.Fprintf(w, "Package defaults: %d top-level keys\n", len(resolved.Values))
		fmt.Fprintf(w, "Value files:      %d\n", len(valueFiles))
		fmt.Fprintf(w, "Set overrides:    %d\n", len(sets))
		fmt.Fprintf(w, "Final values:     %d top-level keys\n", len(mergedValues))
		fmt.Fprintln(w)

		fmt.Fprintln(w, "=== FINAL VALUES ===")
		valOut, valErr := FormatYAML(mergedValues)
		if nil != valErr {
			return valErr
		}
		fmt.Fprint(w, valOut)
		fmt.Fprintln(w)
	}

	// Step 3: Build render context
	ctx := &engine.RenderContext{
		Values: mergedValues,
		Package: map[string]any{
			"name":       resolved.Metadata.Name,
			"version":    resolved.Metadata.Version,
			"appVersion": resolved.Metadata.AppVersion,
		},
		Release: map[string]any{
			"name":      resolved.Metadata.Name,
			"namespace": namespace,
			"revision":  1,
			"isUpgrade": false,
			"isInstall": true,
		},
		Capabilities: map[string]any{},
		Files:        resolved.Files,
	}

	if trace {
		fmt.Fprintln(w, "=== TEMPLATE FILES ===")
		templateNames := sortedKeys(resolved.Templates)
		for _, name := range templateNames {
			fmt.Fprintf(w, "  - %s\n", name)
		}
		fmt.Fprintln(w)

		partialCount := len(resolved.Partials)
		if 0 < partialCount {
			fmt.Fprintf(w, "Partials: %d defined\n", partialCount)
			fmt.Fprintln(w)
		}

		hookCount := len(resolved.Hooks)
		if 0 < hookCount {
			fmt.Fprintf(w, "Hooks: %d files\n", hookCount)
			hookNames := sortedKeys(resolved.Hooks)
			for _, name := range hookNames {
				fmt.Fprintf(w, "  - %s\n", name)
			}
			fmt.Fprintln(w)
		}
	}

	// Step 4: Render templates
	eng := engine.New()
	output, err := eng.Render(resolved.Templates, resolved.Partials, ctx)
	if nil != err {
		return err
	}

	if trace {
		fmt.Fprintln(w, "=== RENDERED OUTPUT ===")
	}

	fmt.Fprint(w, output)

	// Summary
	if !trace {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "---")
		fmt.Fprintf(w, "Package:    %s-%s\n", resolved.Metadata.Name, resolved.Metadata.Version)
		fmt.Fprintf(w, "Templates:  %d\n", len(resolved.Templates))
		fmt.Fprintf(w, "Values:     %d top-level keys\n", len(mergedValues))

		hookCount := len(resolved.Hooks)
		if 0 < hookCount {
			fmt.Fprintf(w, "Hooks:      %d\n", hookCount)
		}

		warnings := collectWarnings(resolved)
		for _, warning := range warnings {
			fmt.Fprintf(w, "WARNING: %s\n", warning)
		}
	}

	return nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func collectWarnings(resolved *layer.ResolvedPackage) []string {
	warnings := make([]string, 0)
	if "" == resolved.Metadata.AppVersion {
		warnings = append(warnings, "no appVersion set in package metadata")
	}
	if _, hasNotes := resolved.Templates["notes.yaml"]; !hasNotes {
		warnings = append(warnings, "no notes.yaml template found")
	}
	if 0 == len(resolved.Templates) {
		warnings = append(warnings, "package contains no templates")
	}
	return warnings
}
