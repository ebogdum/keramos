package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	keramoserr "github.com/ebogdum/keramos/internal/errors"
	"github.com/spf13/cobra"
)

// newInitCommand scaffolds a new keramos package from a built-in template.
//
// Usage:
//
//	keramos init webapp myapp
//	keramos init batch myjob
//	keramos init operator my-operator
func newInitCommand() *cobra.Command {
	var (
		template string
		dest     string
	)
	cmd := &cobra.Command{
		Use:   "init <name>",
		Short: "Scaffold a new keramos package from a built-in template",
		Long: `Create a new keramos package directory with a minimal layout: keramos.yaml,
values.yaml, values.schema.json, templates/, and tests/. Several built-in
templates are available:

  webapp     A web application backed by a Deployment + Service + ConfigMap
  batch      A Job-based batch worker
  operator   A controller package with a CustomResourceDefinition
  blank      The smallest valid keramos package
`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if "" == strings.TrimSpace(name) {
				return keramoserr.NewError(keramoserr.ErrCLIValidation, "package name is required")
			}
			tplName := strings.ToLower(template)
			files, ok := initTemplates[tplName]
			if !ok {
				return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
					"unknown template %q (available: webapp, batch, operator, blank)", template)
			}
			target := filepath.Join(dest, name)
			if _, err := os.Stat(target); nil == err {
				return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
					"target directory %q already exists", target)
			}
			for path, content := range files {
				full := filepath.Join(target, path)
				if mkErr := os.MkdirAll(filepath.Dir(full), 0o755); nil != mkErr {
					return keramoserr.WrapError(keramoserr.ErrInternal, "create dir", mkErr)
				}
				rendered := strings.ReplaceAll(content, "{{NAME}}", name)
				if writeErr := os.WriteFile(full, []byte(rendered), 0o644); nil != writeErr {
					return keramoserr.WrapError(keramoserr.ErrInternal, "write file", writeErr)
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Initialised %s package at %s\n", tplName, target)
			fmt.Fprintf(cmd.OutOrStdout(), "Next:\n  cd %s\n  keramos lint .\n  keramos template . -o yaml\n", target)
			return nil
		},
	}
	cmd.Flags().StringVarP(&template, "template", "t", "blank", "template name: webapp, batch, operator, blank")
	cmd.Flags().StringVar(&dest, "dest", ".", "directory to create the package in (default: current directory)")
	return cmd
}

// initTemplates maps a template name to a map of relative-path → file content.
// `{{NAME}}` is substituted with the package name at write time. Templates
// are intentionally minimal; users extend them.
var initTemplates = map[string]map[string]string{
	"blank": {
		"keramos.yaml": `apiVersion: keramos/v1
name: {{NAME}}
version: 0.1.0
description: A keramos package
`,
		"values.yaml": "# Add values here\n",
		// A minimal but valid placeholder template so the package lints
		// cleanly under --strict; replace it with your own resources.
		"templates/configmap.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: ${release.name}
data:
  greeting: hello from ${release.name}
`,
	},
	"webapp": {
		"keramos.yaml": `apiVersion: keramos/v1
name: {{NAME}}
version: 0.1.0
appVersion: "1.0.0"
description: A web application
environments:
  dev:
    values:
      replicas: 1
  prod:
    inherits: dev
    values:
      replicas: 3
`,
		"values.yaml": `replicas: 1
image:
  repository: nginx
  tag: latest
service:
  port: 80
config:
  greeting: hello
`,
		"values.schema.json": `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "replicas": { "type": "integer", "minimum": 1 },
    "image": {
      "type": "object",
      "required": ["repository"],
      "properties": {
        "repository": { "type": "string" },
        "tag": { "type": "string" }
      }
    },
    "service": {
      "type": "object",
      "properties": {
        "port": { "type": "integer", "minimum": 1, "maximum": 65535 }
      }
    }
  }
}
`,
		"templates/deployment.yaml": `apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${release.name}
spec:
  replicas: ${values.replicas}
  selector:
    matchLabels:
      app: ${release.name}
  template:
    metadata:
      labels:
        app: ${release.name}
    spec:
      containers:
        - name: app
          image: "${values.image.repository}:${values.image.tag}"
          ports:
            - containerPort: ${values.service.port}
          env:
            - name: GREETING
              valueFrom:
                configMapKeyRef:
                  name: ${release.name}
                  key: greeting
`,
		"templates/service.yaml": `apiVersion: v1
kind: Service
metadata:
  name: ${release.name}
spec:
  selector:
    app: ${release.name}
  ports:
    - port: ${values.service.port}
      targetPort: ${values.service.port}
`,
		"templates/configmap.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: ${release.name}
data:
  greeting: ${values.config.greeting}
`,
		"tests/connection.yaml": `apiVersion: v1
kind: Pod
metadata:
  name: ${release.name}-test
spec:
  restartPolicy: Never
  containers:
    - name: probe
      image: busybox:1.36
      command: ["wget", "-O-", "${release.name}:${values.service.port}"]
`,
	},
	"batch": {
		"keramos.yaml": `apiVersion: keramos/v1
name: {{NAME}}
version: 0.1.0
description: A batch job
`,
		"values.yaml": `image:
  repository: busybox
  tag: "1.36"
schedule: "*/5 * * * *"
parallelism: 1
`,
		"templates/cronjob.yaml": `apiVersion: batch/v1
kind: CronJob
metadata:
  name: ${release.name}
spec:
  schedule: ${values.schedule}
  jobTemplate:
    spec:
      parallelism: ${values.parallelism}
      template:
        spec:
          restartPolicy: OnFailure
          containers:
            - name: worker
              image: "${values.image.repository}:${values.image.tag}"
              command: ["/bin/sh", "-c", "echo job ran at $(date)"]
`,
	},
	"operator": {
		"keramos.yaml": `apiVersion: keramos/v1
name: {{NAME}}
version: 0.1.0
description: A Kubernetes operator with a CRD
`,
		"values.yaml": `image:
  repository: example/operator
  tag: latest
`,
		"crds/widgets.yaml": `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.example.com
spec:
  group: example.com
  scope: Namespaced
  names:
    plural: widgets
    singular: widget
    kind: Widget
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
                replicas: { type: integer }
`,
		"templates/deployment.yaml": `apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${release.name}-controller
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ${release.name}-controller
  template:
    metadata:
      labels:
        app: ${release.name}-controller
    spec:
      containers:
        - name: controller
          image: "${values.image.repository}:${values.image.tag}"
`,
	},
}
