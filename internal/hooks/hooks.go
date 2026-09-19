package hooks

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	keramoserr "github.com/ebogdum/keramos/v2/internal/errors"
	"github.com/ebogdum/keramos/v2/internal/kube"
	"github.com/ebogdum/keramos/v2/internal/logger"
	"github.com/ebogdum/keramos/v2/internal/release"
	// yaml.v3 is safe for untrusted input: unlike Python's yaml.load(), Go's yaml.v3
	// does not support arbitrary object instantiation or code execution during
	// deserialization. All values are decoded into standard Go types.
	"gopkg.in/yaml.v3"
)

// HookType represents the lifecycle event.
type HookType string

const (
	PreInstall   HookType = "pre-install"
	PostInstall  HookType = "post-install"
	PreUpgrade   HookType = "pre-upgrade"
	PostUpgrade  HookType = "post-upgrade"
	PreDelete    HookType = "pre-delete"
	PostDelete   HookType = "post-delete"
	PreRollback  HookType = "pre-rollback"
	PostRollback HookType = "post-rollback"
)

const (
	defaultHookTimeout = 5 * time.Minute
)

// Hook represents a parsed hook from the hooks/ directory.
type Hook struct {
	Type         HookType
	Weight       int
	DeletePolicy string // "hook-succeeded", "hook-failed", "before-hook-creation"
	Timeout      time.Duration
	Manifest     string // rendered K8s manifest
}

// hookDirective is the structure parsed from the $hook field in hook files.
type hookDirective struct {
	Weight       int    `yaml:"weight"`
	DeletePolicy string `yaml:"deletePolicy"`
	Timeout      string `yaml:"timeout"`
	Phase        string `yaml:"phase"` // optional: lifecycle phase (pre-install, post-upgrade, ...) when filename does not convey it
}

// ParseHooks processes rendered hook files and extracts Hook metadata.
// Hook files are named by lifecycle event (pre-install.yaml, post-upgrade.yaml).
// Each file can contain a $hook directive with weight, deletePolicy, timeout.
func ParseHooks(hookFiles map[string]string) ([]Hook, error) {
	hooks := make([]Hook, 0, len(hookFiles))

	filenames := make([]string, 0, len(hookFiles))
	for filename := range hookFiles {
		filenames = append(filenames, filename)
	}
	sort.Strings(filenames)

	for _, filename := range filenames {
		content := hookFiles[filename]
		directive, manifest, err := extractDirective(content)
		if nil != err {
			return nil, keramoserr.WrapErrorf(keramoserr.ErrKube, err, "failed to parse hook directive in %s", filename)
		}
		// Phase resolution: filename convention first (pre-install.yaml,
		// post-upgrade.yaml...). Fall back to the $hook directive's
		// `phase:` field for files named freely. Free-form names paired
		// with a $hook directive let users group hooks however they like.
		hookType, fnameErr := hookTypeFromFilename(filename)
		if nil != fnameErr {
			if "" != directive.Phase {
				if t, dErr := hookTypeFromFilename(directive.Phase + ".yaml"); nil == dErr {
					hookType = t
				} else {
					logger.Warn("skipping hook file %s: unrecognized name and unparseable $hook.phase=%q",
						filename, directive.Phase)
					continue
				}
			} else {
				logger.Warn("skipping hook file with unrecognized name: %s", filename)
				continue
			}
		}

		h := Hook{
			Type:         hookType,
			Manifest:     manifest,
			Weight:       directive.Weight,
			DeletePolicy: directive.DeletePolicy,
			Timeout:      defaultHookTimeout,
		}

		if "" != directive.Timeout {
			dur, parseErr := time.ParseDuration(directive.Timeout)
			if nil != parseErr {
				return nil, keramoserr.WrapErrorf(keramoserr.ErrKube, parseErr, "invalid timeout in hook %s", filename)
			}
			h.Timeout = dur
		}

		hooks = append(hooks, h)
	}

	sort.SliceStable(hooks, func(i, j int) bool {
		return hooks[i].Weight < hooks[j].Weight
	})

	return hooks, nil
}

// ExecuteHooks runs hooks of the given type using the K8s client.
// Hooks are sorted by weight (ascending) and executed sequentially.
// To bound a hook's timeout independent of the chart-declared value, use
// ExecuteHooksWithTimeout.
func ExecuteHooks(client kube.KubeClient, allHooks []Hook, hookType HookType) ([]release.HookResult, error) {
	return ExecuteHooksWithTimeout(client, allHooks, hookType, 0)
}

// ExecuteHooksWithTimeout runs hooks but caps each hook's per-hook timeout
// at `maxTimeout` (0 = use the chart-declared value).
func ExecuteHooksWithTimeout(client kube.KubeClient, allHooks []Hook, hookType HookType, maxTimeout time.Duration) ([]release.HookResult, error) {
	matching := filterByType(allHooks, hookType)
	if 0 == len(matching) {
		return nil, nil
	}

	sort.Slice(matching, func(i, j int) bool {
		return matching[i].Weight < matching[j].Weight
	})

	results := make([]release.HookResult, 0, len(matching))

	for _, h := range matching {
		logger.Debug("executing %s hook (weight=%d)", h.Type, h.Weight)

		resources, parseErr := kube.ParseManifests(h.Manifest)
		if nil != parseErr {
			results = append(results, release.HookResult{
				Name:   fmt.Sprintf("%s-hook", h.Type),
				Kind:   "unknown",
				Status: "failed",
			})
			return results, keramoserr.WrapErrorf(keramoserr.ErrKube, parseErr, "failed to parse hook manifest for %s", h.Type)
		}

		// Handle before-hook-creation delete policy
		if "before-hook-creation" == h.DeletePolicy {
			if delErr := deleteHookResources(client, h.Manifest); nil != delErr {
				return results, delErr
			}
		}

		applyErr := client.ApplyManifests(h.Manifest)
		if nil != applyErr {
			hookName := fmt.Sprintf("%s-hook", h.Type)
			hookKind := "unknown"
			if 0 < len(resources) {
				hookName = resources[0].GetName()
				hookKind = resources[0].GetKind()
			}
			results = append(results, release.HookResult{
				Name:   hookName,
				Kind:   hookKind,
				Status: "failed",
			})
			if "hook-failed" == h.DeletePolicy {
				if delErr := deleteHookResources(client, h.Manifest); nil != delErr {
					logger.Warn("hook-failed cleanup for %s reported: %v", h.Type, delErr)
				}
			}
			return results, keramoserr.WrapErrorf(keramoserr.ErrKube, applyErr, "failed to apply hook %s", h.Type)
		}

		// Wait for Job/Pod completion
		hookFailed := false
		var firstWaitErr error
		for _, res := range resources {
			kind := res.GetKind()
			if "Job" == kind {
				ns := res.GetNamespace()
				if "" == ns {
					ns = client.Namespace()
				}
				effectiveTimeout := h.Timeout
				if 0 < maxTimeout && (0 == effectiveTimeout || maxTimeout < effectiveTimeout) {
					effectiveTimeout = maxTimeout
				}
				if waitErr := client.WaitForJob(ns, res.GetName(), effectiveTimeout); nil != waitErr {
					results = append(results, release.HookResult{
						Name:   res.GetName(),
						Kind:   kind,
						Status: "failed",
					})
					hookFailed = true
					if nil == firstWaitErr {
						firstWaitErr = keramoserr.WrapErrorf(keramoserr.ErrKube, waitErr, "hook job %s failed", res.GetName())
					}
					continue
				}
			}

			results = append(results, release.HookResult{
				Name:   res.GetName(),
				Kind:   kind,
				Status: "succeeded",
			})
		}

		if hookFailed {
			if "hook-failed" == h.DeletePolicy {
				if delErr := deleteHookResources(client, h.Manifest); nil != delErr {
					logger.Warn("hook-failed cleanup for %s reported: %v", h.Type, delErr)
				}
			}
			return results, firstWaitErr
		}

		// Handle hook-succeeded delete policy
		if "hook-succeeded" == h.DeletePolicy {
			if delErr := deleteHookResources(client, h.Manifest); nil != delErr {
				return results, delErr
			}
		}
	}

	return results, nil
}

// deleteHookResources deletes hook resources, treating not-found as success.
// Real errors (permission denied, network failure) are propagated.
func deleteHookResources(client kube.KubeClient, manifest string) error {
	delErr := client.DeleteManifests(manifest)
	if nil == delErr {
		return nil
	}
	// DeleteManifests in kube.Client already treats not-found as success,
	// so any error returned here is a real failure that should propagate.
	errMsg := delErr.Error()
	if strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "NotFound") {
		return nil
	}
	return keramoserr.WrapError(keramoserr.ErrKube, "failed to delete hook resources", delErr)
}

func filterByType(allHooks []Hook, hookType HookType) []Hook {
	result := make([]Hook, 0)
	for _, h := range allHooks {
		if h.Type == hookType {
			result = append(result, h)
		}
	}
	return result
}

func hookTypeFromFilename(filename string) (HookType, error) {
	name := strings.TrimSuffix(filename, ".yaml")
	name = strings.TrimSuffix(name, ".yml")

	// Allow weight suffixes like "pre-install-10.yaml"
	validTypes := []HookType{
		PreInstall, PostInstall,
		PreUpgrade, PostUpgrade,
		PreDelete, PostDelete,
		PreRollback, PostRollback,
	}
	for _, ht := range validTypes {
		if strings.HasPrefix(name, string(ht)) {
			return ht, nil
		}
	}

	return "", fmt.Errorf("unrecognized hook type: %s", filename)
}

func extractDirective(content string) (hookDirective, string, error) {
	var directive hookDirective

	// Decode every document so multi-document hook files are preserved.
	// yaml.v3 is safe for untrusted input: no arbitrary object instantiation.
	dec := yaml.NewDecoder(strings.NewReader(content))
	docs := make([]map[string]any, 0)
	directiveFound := false
	for {
		var doc map[string]any
		decErr := dec.Decode(&doc)
		if nil != decErr {
			if decErr.Error() == "EOF" {
				break
			}
			// Not parseable as map docs; return content as-is.
			return directive, content, nil
		}
		if nil == doc {
			continue
		}

		if hookRaw, exists := doc["$hook"]; exists {
			if directiveFound {
				return directive, "", fmt.Errorf("multiple $hook directives in the same hook file are not allowed")
			}
			// Short-hand string form: `$hook: pre-install` is equivalent to
			// `$hook: {phase: pre-install}`. The bracket form supports
			// weight/deletePolicy/timeout as well.
			if phaseStr, ok := hookRaw.(string); ok {
				directive.Phase = phaseStr
				directiveFound = true
				delete(doc, "$hook")
				// Also strip sibling $hookWeight / $hookDeletePolicy /
				// $hookTimeout keys at the top level — they're keramos's
				// flat shorthand for the bracket form.
				if w, ok := doc["$hookWeight"]; ok {
					switch v := w.(type) {
					case int:
						directive.Weight = v
					case float64:
						directive.Weight = int(v)
					}
					delete(doc, "$hookWeight")
				}
				if dp, ok := doc["$hookDeletePolicy"].(string); ok {
					directive.DeletePolicy = dp
					delete(doc, "$hookDeletePolicy")
				}
				if t, ok := doc["$hookTimeout"].(string); ok {
					directive.Timeout = t
					delete(doc, "$hookTimeout")
				}
				if 0 == len(doc) {
					continue
				}
				docs = append(docs, doc)
				continue
			}
			if hookMap, ok := hookRaw.(map[string]any); ok {
				if w, ok := hookMap["weight"]; ok {
					switch v := w.(type) {
					case int:
						directive.Weight = v
					case float64:
						directive.Weight = int(v)
					case string:
						parsed, parseErr := strconv.Atoi(v)
						if nil != parseErr {
							return directive, "", fmt.Errorf("invalid hook weight: %s", v)
						}
						directive.Weight = parsed
					}
				}
				if dp, ok := hookMap["deletePolicy"].(string); ok {
					directive.DeletePolicy = dp
				}
				if t, ok := hookMap["timeout"].(string); ok {
					directive.Timeout = t
				}
				if ph, ok := hookMap["phase"].(string); ok {
					directive.Phase = ph
				}
				directiveFound = true
			}
			delete(doc, "$hook")
			if 0 == len(doc) {
				continue
			}
		}

		docs = append(docs, doc)
	}

	if !directiveFound {
		return directive, content, nil
	}

	parts := make([]string, 0, len(docs))
	for _, d := range docs {
		out, err := yaml.Marshal(d)
		if nil != err {
			return directive, "", fmt.Errorf("failed to re-serialize hook manifest: %w", err)
		}
		parts = append(parts, string(out))
	}
	return directive, strings.Join(parts, "---\n"), nil
}
