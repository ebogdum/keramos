package action

import (
	"fmt"
	"os"
	"path/filepath"
)

// Create scaffolds a new keramos package in the given directory.
//
// `name` may be an absolute or relative path; the package's `metadata.name`
// becomes the basename, and the directory is created at the supplied path.
// This lets `keramos create scratch/c-1` produce a valid package whose
// `name: c-1` field matches the lint regex.
func Create(name string, dir string) error {
	targetDir := name
	if !filepath.IsAbs(name) {
		targetDir = filepath.Join(dir, name)
	}
	pkgName := filepath.Base(targetDir)
	if _, err := os.Stat(targetDir); nil == err {
		return fmt.Errorf("directory %q already exists", targetDir)
	}

	dirs := []string{
		targetDir,
		filepath.Join(targetDir, "templates"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); nil != err {
			return fmt.Errorf("failed to create directory %s: %w", d, err)
		}
	}

	files := map[string]string{
		"keramos.yaml":               keramosYAML(pkgName),
		"values.yaml":             valuesYAML(pkgName),
		"templates/deployment.yaml": deploymentTemplate(),
		"templates/service.yaml":    serviceTemplate(),
		"templates/_helpers.yaml":   helpersTemplate(),
		"templates/notes.yaml":      notesTemplate(pkgName),
		".keramosignore":               keramosignoreContent(),
	}

	for relPath, content := range files {
		fullPath := filepath.Join(targetDir, relPath)
		if err := os.WriteFile(fullPath, []byte(content), 0644); nil != err {
			return fmt.Errorf("failed to write %s: %w", relPath, err)
		}
	}

	return nil
}

func keramosYAML(name string) string {
	return fmt.Sprintf(`apiVersion: keramos/v1
name: %s
version: 0.1.0
description: A keramos package for %s
`, name, name)
}

func valuesYAML(name string) string {
	return fmt.Sprintf(`name: %s
replicaCount: 1
image:
  repository: nginx
  tag: latest
service:
  port: 80
`, name)
}

func deploymentTemplate() string {
	return `apiVersion: apps/v1
kind: Deployment
metadata:
  name: "${values.name}"
  labels:
    app: "${values.name}"
spec:
  replicas: ${values.replicaCount}
  selector:
    matchLabels:
      app: "${values.name}"
  template:
    metadata:
      labels:
        app: "${values.name}"
    spec:
      containers:
        - name: "${values.name}"
          image: "${values.image.repository}:${values.image.tag}"
          ports:
            - containerPort: ${values.service.port}
`
}

func serviceTemplate() string {
	return `apiVersion: v1
kind: Service
metadata:
  name: "${values.name}"
  labels:
    app: "${values.name}"
spec:
  type: ClusterIP
  ports:
    - port: ${values.service.port}
      targetPort: ${values.service.port}
      protocol: TCP
  selector:
    app: "${values.name}"
`
}

func helpersTemplate() string {
	return `# Common labels partial
# Include via $include in your templates
common-labels:
  app: "${values.name}"
  version: "${package.version}"
`
}

func notesTemplate(name string) string {
	return fmt.Sprintf(`message: |
  %s has been installed successfully.
  Namespace: ${release.namespace}
  Run "kubectl get deployments" to verify.
`, name)
}

func keramosignoreContent() string {
	return `# Patterns to ignore when packaging
.git
.gitignore
.keramosignore
*.swp
*.bak
*~
.DS_Store
`
}
