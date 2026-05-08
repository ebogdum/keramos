package action

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// extractNotes separates notes documents from K8s manifests.
// A notes document is a YAML document with a single "message" key.
// Returns the cleaned manifest string and the extracted notes message.
func extractNotes(rendered string) (string, string, error) {
	if "" == rendered {
		return "", "", nil
	}

	docs := strings.Split(rendered, "---\n")
	kept := make([]string, 0, len(docs))
	var notes string

	for _, doc := range docs {
		trimmed := strings.TrimSpace(doc)
		if "" == trimmed {
			continue
		}

		msg, isNotes, err := parseNotesDoc(trimmed)
		if nil != err {
			// Not valid YAML map — keep as manifest
			kept = append(kept, doc)
			continue
		}
		if isNotes {
			notes = msg
			continue
		}
		kept = append(kept, doc)
	}

	manifest := strings.Join(kept, "---\n")
	return manifest, notes, nil
}

// parseNotesDoc checks whether a YAML document is a notes document
// (a map with exactly one key "message" whose value is a string).
// Returns the message, whether it is a notes doc, and any error.
func parseNotesDoc(doc string) (string, bool, error) {
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(doc), &parsed); nil != err {
		return "", false, err
	}

	if 1 != len(parsed) {
		return "", false, nil
	}

	val, ok := parsed["message"]
	if !ok {
		return "", false, nil
	}

	msg, ok := val.(string)
	if !ok {
		return "", false, nil
	}

	return msg, true, nil
}
