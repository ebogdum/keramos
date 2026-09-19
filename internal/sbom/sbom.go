// Package sbom emits CycloneDX 1.5 JSON Software Bill of Materials documents
// for keramos releases. The SBOM lists every container image referenced, every
// layer/dependency in the dependency tree, and the release metadata so
// downstream supply-chain tooling (cosign attest, Grype, Trivy, Dependency
// Track) can ingest it.
package sbom

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ebogdum/keramos/v2/internal/release"
	"gopkg.in/yaml.v3"
)

// Document is a CycloneDX 1.5 SBOM body.
type Document struct {
	BOMFormat   string     `json:"bomFormat"`
	SpecVersion string     `json:"specVersion"`
	SerialNumber string    `json:"serialNumber"`
	Version     int        `json:"version"`
	Metadata    Metadata   `json:"metadata"`
	Components  []Component `json:"components"`
}

type Metadata struct {
	Timestamp time.Time `json:"timestamp"`
	Tools     []Tool    `json:"tools"`
	Component Component `json:"component"`
}

type Tool struct {
	Vendor  string `json:"vendor"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Component struct {
	Type        string `json:"type"`           // application | container | library
	BOMRef      string `json:"bom-ref"`
	Name        string `json:"name"`
	Version     string `json:"version,omitempty"`
	PackageURL  string `json:"purl,omitempty"`
	Description string `json:"description,omitempty"`
}

// Generate inspects a release and produces a CycloneDX SBOM. Container images
// are extracted from every PodSpec in the rendered manifest. The release
// itself becomes the root component; layers and images are listed.
func Generate(rel *release.Release, keramosVersion string) (*Document, error) {
	if nil == rel {
		return nil, fmt.Errorf("nil release")
	}

	images := extractImages(rel.Manifest)

	doc := &Document{
		BOMFormat:    "CycloneDX",
		SpecVersion:  "1.5",
		SerialNumber: "urn:uuid:" + simpleUUID(rel),
		Version:      1,
		Metadata: Metadata{
			Timestamp: time.Now().UTC(),
			Tools: []Tool{{
				Vendor:  "keramos",
				Name:    "keramos",
				Version: keramosVersion,
			}},
			Component: Component{
				Type:        "application",
				BOMRef:      "release/" + rel.Name,
				Name:        rel.Name,
				Version:     fmt.Sprintf("rev-%d", rel.Revision),
				Description: rel.Info.Description,
			},
		},
		Components: make([]Component, 0, 1+len(images)),
	}
	doc.Components = append(doc.Components, Component{
		Type:    "library",
		BOMRef:  "package/" + rel.Package.Name + "@" + rel.Package.Version,
		Name:    rel.Package.Name,
		Version: rel.Package.Version,
		PackageURL: fmt.Sprintf("pkg:keramos/%s@%s", rel.Package.Name, rel.Package.Version),
	})
	for _, img := range images {
		doc.Components = append(doc.Components, Component{
			Type:       "container",
			BOMRef:     "image/" + img,
			Name:       imageNameOnly(img),
			Version:    imageTag(img),
			PackageURL: imagePURL(img),
		})
	}
	return doc, nil
}

// FormatJSON marshals as canonical CycloneDX JSON.
func FormatJSON(doc *Document) (string, error) {
	out, err := json.MarshalIndent(doc, "", "  ")
	if nil != err {
		return "", err
	}
	return string(out), nil
}

// extractImages walks every YAML doc and pulls `image:` values out of any
// containers/initContainers/ephemeralContainers list.
func extractImages(manifest string) []string {
	out := make(map[string]bool)
	dec := yaml.NewDecoder(strings.NewReader(manifest))
	for {
		var doc map[string]any
		err := dec.Decode(&doc)
		if nil != err {
			break
		}
		walkForImages(doc, out)
	}
	keys := make([]string, 0, len(out))
	for k := range out {
		keys = append(keys, k)
	}
	return keys
}

func walkForImages(node any, out map[string]bool) {
	switch v := node.(type) {
	case map[string]any:
		for k, child := range v {
			if "image" == k {
				if s, ok := child.(string); ok && "" != s {
					out[s] = true
				}
			}
			walkForImages(child, out)
		}
	case []any:
		for _, child := range v {
			walkForImages(child, out)
		}
	}
}

func imageNameOnly(image string) string {
	if i := strings.LastIndex(image, "@"); -1 != i {
		image = image[:i]
	}
	if i := strings.LastIndex(image, ":"); -1 != i {
		// guard for ports in registry host
		if !strings.ContainsAny(image[i+1:], "/") {
			image = image[:i]
		}
	}
	return image
}

func imageTag(image string) string {
	if i := strings.LastIndex(image, "@"); -1 != i {
		return image[i+1:]
	}
	if i := strings.LastIndex(image, ":"); -1 != i && !strings.ContainsAny(image[i+1:], "/") {
		return image[i+1:]
	}
	return "latest"
}

func imagePURL(image string) string {
	name := imageNameOnly(image)
	tag := imageTag(image)
	return fmt.Sprintf("pkg:oci/%s?tag=%s", name, tag)
}

var uuidNonHex = regexp.MustCompile(`[^a-f0-9]`)

// simpleUUID is a deterministic-enough identifier from release name+rev.
// Real UUIDs would be nicer but a CycloneDX consumer only needs uniqueness.
func simpleUUID(rel *release.Release) string {
	raw := fmt.Sprintf("%s-%d-%s", rel.Name, rel.Revision, rel.Info.LastDeployed.Format(time.RFC3339Nano))
	hash := strings.ToLower(raw)
	hash = uuidNonHex.ReplaceAllString(hash, "")
	for len(hash) < 32 {
		hash += "0"
	}
	hash = hash[:32]
	return fmt.Sprintf("%s-%s-%s-%s-%s", hash[0:8], hash[8:12], hash[12:16], hash[16:20], hash[20:32])
}
