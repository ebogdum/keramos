{{- define "myapp.name" -}}
{{- .Values.nameOverride | default .Chart.Name -}}
{{- end -}}

{{- define "myapp.fullname" -}}
{{- .Release.Name }}-{{ .Chart.Name }}
{{- end -}}
