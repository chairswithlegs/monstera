{{/*
Chart name
*/}}
{{- define "monstera.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Full name (release + chart)
*/}}
{{- define "monstera.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Server full name
*/}}
{{- define "monstera.server.fullname" -}}
{{- printf "%s-server" (include "monstera.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
UI full name
*/}}
{{- define "monstera.ui.fullname" -}}
{{- printf "%s-ui" (include "monstera.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "monstera.labels" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
app.kubernetes.io/name: {{ include "monstera.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Server image
*/}}
{{- define "monstera.server.image" -}}
{{- printf "%s:%s" .Values.server.image.repository .Values.server.image.tag }}
{{- end }}

{{/*
UI image
*/}}
{{- define "monstera.ui.image" -}}
{{- printf "%s:%s" .Values.ui.image.repository .Values.ui.image.tag }}
{{- end }}

{{/*
Media PVC name (for local storage)
*/}}
{{- define "monstera.server.mediaPvcName" -}}
{{- printf "%s-media" (include "monstera.server.fullname" .) }}
{{- end }}
