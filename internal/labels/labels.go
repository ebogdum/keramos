// Package labels centralises the label keramos writes onto every resource
// it creates. The single label is the source of truth for "keramos made
// this": purge, list, drift, and any future audit tooling all key on
// it. We use a label (not an annotation) because it is selector-able,
// so cluster-wide queries like `managedBy=keramos` are O(label index)
// instead of O(list-then-scan-annotations).
//
// Resources that physically cannot carry a label (none in the core API
// at the time of writing) would be marked with the equivalent
// annotation instead — see ManagedByAnnotation.
package labels

// ManagedByLabel is the canonical key. Every resource keramos applies or
// creates carries this label set to ManagedByValue. A separate legacy
// "owner" key exists on release-storage Secrets for backwards
// compatibility with releases written by older keramos versions; new code
// should not write the legacy key.
const (
	ManagedByLabel      = "managedBy"
	ManagedByValue      = "keramos"
	ManagedByAnnotation = "keramos.sh/managed-by"
)

// IsKeramosManaged returns true if the given labels/annotations map
// indicates the object was created by keramos. Callers pass either map;
// the helper checks both keys so resources marked by annotation (when
// label was impossible) are still recognised.
func IsKeramosManaged(lbls, annos map[string]string) bool {
	if nil != lbls && ManagedByValue == lbls[ManagedByLabel] {
		return true
	}
	if nil != annos && ManagedByValue == annos[ManagedByAnnotation] {
		return true
	}
	return false
}

// Selector returns the canonical label selector used by purge, list,
// and discovery code — always `managedBy=keramos`.
func Selector() string {
	return ManagedByLabel + "=" + ManagedByValue
}
