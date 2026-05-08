package action

import (
	"os"

	keramoserr "github.com/ebogdum/keramos/internal/errors"
	"github.com/ebogdum/keramos/internal/pkg"
	"gopkg.in/yaml.v3"
)

// writeEnvValuesFile materialises the environment's inline `values:` block
// into a tempfile so it can participate as a normal values-file layer in
// resolution. Returns the file path; the caller is expected to leave the
// file in place for the duration of the render (the OS reaps temp dirs).
func writeEnvValuesFile(values map[string]any) (string, error) {
	tmp, err := os.CreateTemp("", "keramos-env-*.yaml")
	if nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrInternal, "create env values tempfile", err)
	}
	defer tmp.Close()
	data, mErr := yaml.Marshal(values)
	if nil != mErr {
		os.Remove(tmp.Name())
		return "", keramoserr.WrapError(keramoserr.ErrInternal, "marshal env values", mErr)
	}
	if _, wErr := tmp.Write(data); nil != wErr {
		os.Remove(tmp.Name())
		return "", keramoserr.WrapError(keramoserr.ErrInternal, "write env values tempfile", wErr)
	}
	return tmp.Name(), nil
}

// EnvOverlay is the result of resolving an environment from keramos.yaml.
// Callers pass it through their own values-resolution pipeline.
type EnvOverlay struct {
	Profile    string
	Namespace  string
	ValueFiles []string
	SetJSON    []string
}

// ResolveEnvironmentOverlay returns the values an environment named `env`
// contributes (inheritance honoured). Used by `keramos template --env` and
// other render-time entry points that don't go through Install/Upgrade
// option structs.
func ResolveEnvironmentOverlay(packagePath, envName string) (*EnvOverlay, error) {
	if "" == envName {
		return &EnvOverlay{}, nil
	}
	meta, err := pkg.LoadPackageMetadata(packagePath)
	if nil != err {
		return nil, err
	}
	env, err := meta.ResolveEnvironment(envName)
	if nil != err {
		return nil, keramoserr.WrapErrorf(keramoserr.ErrCLIValidation, err, "environment %q", envName)
	}
	out := &EnvOverlay{
		Profile:    env.Profile,
		Namespace:  env.Namespace,
		ValueFiles: append([]string(nil), env.ValueFiles...),
	}
	if 0 < len(env.Values) {
		path, wErr := writeEnvValuesFile(env.Values)
		if nil != wErr {
			return nil, wErr
		}
		// Prepend so env values are the lowest-precedence among --values
		// overrides; package defaults still come first.
		out.ValueFiles = append([]string{path}, out.ValueFiles...)
	}
	return out, nil
}

// applyEnvironmentToInstall folds `keramos.yaml#environments[opts.Environment]`
// into install options. Inheritance is honoured. Inline `values` become a
// synthesised --set-json entry so they participate in the trace.
func applyEnvironmentToInstall(packagePath string, opts *InstallOptions) error {
	meta, err := pkg.LoadPackageMetadata(packagePath)
	if nil != err {
		return err
	}
	env, err := meta.ResolveEnvironment(opts.Environment)
	if nil != err {
		return keramoserr.WrapErrorf(keramoserr.ErrCLIValidation, err,
			"environment %q", opts.Environment)
	}
	// Profile/namespace come from the env when not already overridden on the CLI.
	if "" == opts.Profile && "" != env.Profile {
		opts.Profile = env.Profile
	}
	if "" == opts.Namespace && "" != env.Namespace {
		opts.Namespace = env.Namespace
	}
	if 0 < len(env.ValueFiles) {
		opts.ValueFiles = append(env.ValueFiles, opts.ValueFiles...)
	}
	if 0 < len(env.Values) {
		path, wErr := writeEnvValuesFile(env.Values)
		if nil != wErr {
			return wErr
		}
		opts.ValueFiles = append([]string{path}, opts.ValueFiles...)
	}
	return nil
}

// applyEnvironmentToUpgrade is the upgrade-time analogue.
func applyEnvironmentToUpgrade(packagePath string, opts *UpgradeOptions) error {
	meta, err := pkg.LoadPackageMetadata(packagePath)
	if nil != err {
		return err
	}
	env, err := meta.ResolveEnvironment(opts.Environment)
	if nil != err {
		return keramoserr.WrapErrorf(keramoserr.ErrCLIValidation, err,
			"environment %q", opts.Environment)
	}
	if "" == opts.Profile && "" != env.Profile {
		opts.Profile = env.Profile
	}
	if "" == opts.Namespace && "" != env.Namespace {
		opts.Namespace = env.Namespace
	}
	if 0 < len(env.ValueFiles) {
		opts.ValueFiles = append(env.ValueFiles, opts.ValueFiles...)
	}
	if 0 < len(env.Values) {
		path, wErr := writeEnvValuesFile(env.Values)
		if nil != wErr {
			return wErr
		}
		opts.ValueFiles = append([]string{path}, opts.ValueFiles...)
	}
	return nil
}
