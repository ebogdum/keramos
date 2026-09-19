package release

import (
	"encoding/json"
	"sort"
	"sync"

	keramoserr "github.com/ebogdum/keramos/v2/internal/errors"
)

// cloneRelease returns a deep copy of rel via a JSON round-trip. The cluster-
// backed storages (Secret/ConfigMap/SQL) get this for free because they
// serialise on write and deserialise on read; MemoryStorage holds pointers
// directly, so without an explicit clone the caller's mutations would alias
// the stored Release's maps and slices.
func cloneRelease(rel *Release) (*Release, error) {
	if nil == rel {
		return nil, nil
	}
	data, err := json.Marshal(rel)
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrRelease, "clone release: marshal", err)
	}
	out := &Release{}
	if uErr := json.Unmarshal(data, out); nil != uErr {
		return nil, keramoserr.WrapError(keramoserr.ErrRelease, "clone release: unmarshal", uErr)
	}
	return out, nil
}

// MemoryStorage is an in-process Storage useful for tests, dry runs, and
// air-gapped tooling that should not touch a real cluster.
type MemoryStorage struct {
	mu       sync.RWMutex
	releases map[string]map[int]*Release // name -> revision -> release
}

// NewMemoryStorage returns an empty in-memory Storage.
func NewMemoryStorage() Storage {
	return &MemoryStorage{releases: make(map[string]map[int]*Release)}
}

func (s *MemoryStorage) Create(rel *Release) error {
	if err := enforceMemorySize(rel); nil != err {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	revs, ok := s.releases[rel.Name]
	if !ok {
		revs = make(map[int]*Release)
		s.releases[rel.Name] = revs
	}
	if _, exists := revs[rel.Revision]; exists {
		return keramoserr.NewErrorf(keramoserr.ErrRelease, "release %s v%d already exists", rel.Name, rel.Revision)
	}
	cp, cloneErr := cloneRelease(rel)
	if nil != cloneErr {
		return cloneErr
	}
	revs[rel.Revision] = cp
	return nil
}

// enforceMemorySize mirrors the SecretStorage 1MB cap so a release that fits
// in memory is also storable in a Secret if migrated later. Catching the
// limit at write time avoids lying-via-success in dev environments.
func enforceMemorySize(rel *Release) error {
	encoded, err := Encode(rel)
	if nil != err {
		return err
	}
	if int64(len(encoded)) > maxSecretSize {
		return keramoserr.NewErrorf(keramoserr.ErrRelease,
			"release payload exceeds storage size limit of %d bytes (memory driver enforces parity with Secret driver)", maxSecretSize)
	}
	return nil
}

func (s *MemoryStorage) Update(rel *Release) error {
	if err := enforceMemorySize(rel); nil != err {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	revs, ok := s.releases[rel.Name]
	if !ok {
		return keramoserr.NewErrorf(keramoserr.ErrReleaseNotFound, "release %s not found", rel.Name)
	}
	if _, exists := revs[rel.Revision]; !exists {
		return keramoserr.NewErrorf(keramoserr.ErrReleaseNotFound, "release %s v%d not found", rel.Name, rel.Revision)
	}
	cp, cloneErr := cloneRelease(rel)
	if nil != cloneErr {
		return cloneErr
	}
	revs[rel.Revision] = cp
	return nil
}

func (s *MemoryStorage) Get(name string, revision int) (*Release, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	revs, ok := s.releases[name]
	if !ok {
		return nil, keramoserr.NewErrorf(keramoserr.ErrReleaseNotFound, "release %s not found", name)
	}
	rel, exists := revs[revision]
	if !exists {
		return nil, keramoserr.NewErrorf(keramoserr.ErrReleaseNotFound, "release %s v%d not found", name, revision)
	}
	return cloneRelease(rel)
}

func (s *MemoryStorage) List(_ string) ([]*Release, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Release, 0)
	for _, revs := range s.releases {
		for _, rel := range revs {
			cp, err := cloneRelease(rel)
			if nil != err {
				return nil, err
			}
			out = append(out, cp)
		}
	}
	return out, nil
}

func (s *MemoryStorage) Last(name string) (*Release, error) {
	history, err := s.History(name)
	if nil != err {
		return nil, err
	}
	if 0 == len(history) {
		return nil, keramoserr.NewErrorf(keramoserr.ErrReleaseNotFound, "release %s not found", name)
	}
	return history[len(history)-1], nil
}

func (s *MemoryStorage) History(name string) ([]*Release, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	revs, ok := s.releases[name]
	if !ok {
		return []*Release{}, nil
	}
	out := make([]*Release, 0, len(revs))
	for _, rel := range revs {
		cp := *rel
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Revision < out[j].Revision })
	return out, nil
}

func (s *MemoryStorage) Delete(name string, revision int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	revs, ok := s.releases[name]
	if !ok {
		return nil
	}
	delete(revs, revision)
	if 0 == len(revs) {
		delete(s.releases, name)
	}
	return nil
}
