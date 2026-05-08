// Package workspace orchestrates multiple keramos packages declared in a single
// keramos-workspace.yaml file. Use case: monorepos and platform teams that ship
// dozens of related packages and want one command to install/upgrade/uninstall
// them in dependency order.
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	keramoserr "github.com/ebogdum/keramos/internal/errors"
	"gopkg.in/yaml.v3"
)

// Workspace is the parsed keramos-workspace.yaml.
type Workspace struct {
	APIVersion string         `yaml:"apiVersion"`
	Members    []Member       `yaml:"members"`
	Defaults   MemberDefaults `yaml:"defaults,omitempty"`
}

// Member is one package within the workspace.
type Member struct {
	Name      string   `yaml:"name"`
	Path      string   `yaml:"path"`
	Namespace string   `yaml:"namespace,omitempty"`
	Profile   string   `yaml:"profile,omitempty"`
	DependsOn []string `yaml:"dependsOn,omitempty"`
	Atomic    *bool    `yaml:"atomic,omitempty"`
	Wait      *bool    `yaml:"wait,omitempty"`
}

// MemberDefaults provides workspace-level defaults applied to each member
// unless the member overrides them.
type MemberDefaults struct {
	Namespace string `yaml:"namespace,omitempty"`
	Profile   string `yaml:"profile,omitempty"`
	Atomic    *bool  `yaml:"atomic,omitempty"`
	Wait      *bool  `yaml:"wait,omitempty"`
}

// Load parses keramos-workspace.yaml from the given directory.
func Load(dir string) (*Workspace, error) {
	path := filepath.Join(dir, "keramos-workspace.yaml")
	data, err := os.ReadFile(path)
	if nil != err {
		if os.IsNotExist(err) {
			return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
				"keramos-workspace.yaml not found in %s", dir)
		}
		return nil, keramoserr.WrapError(keramoserr.ErrCLIValidation, "failed to read workspace file", err)
	}
	var ws Workspace
	if yamlErr := yaml.Unmarshal(data, &ws); nil != yamlErr {
		return nil, keramoserr.WrapError(keramoserr.ErrCLIValidation, "invalid workspace yaml", yamlErr)
	}
	if 0 == len(ws.Members) {
		return nil, keramoserr.NewError(keramoserr.ErrCLIValidation, "workspace declares no members")
	}
	for i, m := range ws.Members {
		if "" == m.Name || "" == m.Path {
			return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
				"workspace member[%d] missing name or path", i)
		}
	}
	ws.applyDefaults()
	return &ws, nil
}

// applyDefaults fills empty member fields from workspace-level defaults.
func (w *Workspace) applyDefaults() {
	for i := range w.Members {
		m := &w.Members[i]
		if "" == m.Namespace {
			m.Namespace = w.Defaults.Namespace
		}
		if "" == m.Profile {
			m.Profile = w.Defaults.Profile
		}
		if nil == m.Atomic {
			m.Atomic = w.Defaults.Atomic
		}
		if nil == m.Wait {
			m.Wait = w.Defaults.Wait
		}
	}
}

// TopologicalOrder returns the members sorted so that every dependency comes
// before its dependents. Cycles are reported as an error.
func (w *Workspace) TopologicalOrder() ([]Member, error) {
	byName := make(map[string]*Member, len(w.Members))
	for i := range w.Members {
		byName[w.Members[i].Name] = &w.Members[i]
	}
	visited := make(map[string]int) // 0 unseen, 1 visiting, 2 done
	order := make([]Member, 0, len(w.Members))
	var visit func(name string, stack []string) error
	visit = func(name string, stack []string) error {
		state := visited[name]
		if 2 == state {
			return nil
		}
		if 1 == state {
			return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
				"workspace dependency cycle: %v -> %s", stack, name)
		}
		visited[name] = 1
		m, ok := byName[name]
		if !ok {
			return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
				"workspace member %q references unknown dependency", name)
		}
		// Sort deps for stable output.
		deps := append([]string{}, m.DependsOn...)
		sort.Strings(deps)
		for _, d := range deps {
			if err := visit(d, append(stack, name)); nil != err {
				return err
			}
		}
		visited[name] = 2
		order = append(order, *m)
		return nil
	}
	// Iterate in declared order so two members with no dependency relationship
	// keep their keramos-workspace.yaml order.
	for _, m := range w.Members {
		if err := visit(m.Name, nil); nil != err {
			return nil, err
		}
	}
	return order, nil
}

// Levels returns members grouped by topological depth: every member at
// level N has all of its dependencies in levels 0..N-1. Members within the
// same level have no dependencies between each other and can therefore be
// processed concurrently. Cycles are surfaced as an error (same logic as
// TopologicalOrder).
func (w *Workspace) Levels() ([][]Member, error) {
	byName := make(map[string]*Member, len(w.Members))
	for i := range w.Members {
		byName[w.Members[i].Name] = &w.Members[i]
	}
	for _, m := range w.Members {
		for _, d := range m.DependsOn {
			if _, ok := byName[d]; !ok {
				return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
					"workspace member %q depends on unknown member %q", m.Name, d)
			}
		}
	}
	indeg := make(map[string]int, len(w.Members))
	for _, m := range w.Members {
		seen := map[string]bool{}
		for _, d := range m.DependsOn {
			if seen[d] {
				continue
			}
			seen[d] = true
			indeg[m.Name]++
		}
	}
	levels := make([][]Member, 0)
	processed := 0
	current := make([]string, 0)
	for _, m := range w.Members {
		if 0 == indeg[m.Name] {
			current = append(current, m.Name)
		}
	}
	for 0 < len(current) {
		sort.Strings(current)
		levelMembers := make([]Member, 0, len(current))
		for _, name := range current {
			levelMembers = append(levelMembers, *byName[name])
		}
		levels = append(levels, levelMembers)
		processed += len(current)
		next := make([]string, 0)
		for _, name := range current {
			// Decrement indegree of every member that depends on `name`.
			for _, m := range w.Members {
				for _, d := range m.DependsOn {
					if d == name {
						indeg[m.Name]--
						if 0 == indeg[m.Name] {
							next = append(next, m.Name)
						}
						break
					}
				}
			}
		}
		current = next
	}
	if processed != len(w.Members) {
		stuck := make([]string, 0)
		for n, d := range indeg {
			if 0 < d {
				stuck = append(stuck, n)
			}
		}
		sort.Strings(stuck)
		return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
			"workspace dependency cycle involving: %v", stuck)
	}
	return levels, nil
}

// PackagePath resolves a member's `path` against the workspace directory.
func (m *Member) PackagePath(workspaceDir string) string {
	if filepath.IsAbs(m.Path) {
		return m.Path
	}
	return filepath.Join(workspaceDir, m.Path)
}

// MemberAtomic returns whether atomic mode is requested for a member.
func (m *Member) MemberAtomic() bool {
	if nil == m.Atomic {
		return true // default ON, matching install command
	}
	return *m.Atomic
}

// MemberWait returns whether wait-for-ready is requested for a member.
func (m *Member) MemberWait() bool {
	if nil == m.Wait {
		return true
	}
	return *m.Wait
}

// FormatPlan renders the topo order as text for `keramos workspace plan`.
func FormatPlan(members []Member) string {
	var b []byte
	for i, m := range members {
		b = append(b, []byte(fmt.Sprintf("%d. %s (path=%s, ns=%s, profile=%s)\n",
			i+1, m.Name, m.Path, m.Namespace, m.Profile))...)
	}
	return string(b)
}
