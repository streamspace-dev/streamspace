{{/*
Expand the name of the chart.
*/}}
{{- define "streamspace.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "streamspace.fullname" -}}
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
Create chart name and version as used by the chart label.
*/}}
{{- define "streamspace.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "streamspace.labels" -}}
helm.sh/chart: {{ include "streamspace.chart" . }}
{{ include "streamspace.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- with .Values.commonLabels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "streamspace.selectorLabels" -}}
app.kubernetes.io/name: {{ include "streamspace.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Controller labels
*/}}
{{- define "streamspace.controller.labels" -}}
{{ include "streamspace.labels" . }}
app.kubernetes.io/component: controller
{{- end }}

{{/*
API labels
*/}}
{{- define "streamspace.api.labels" -}}
{{ include "streamspace.labels" . }}
app.kubernetes.io/component: api
{{- end }}

{{/*
UI labels
*/}}
{{- define "streamspace.ui.labels" -}}
{{ include "streamspace.labels" . }}
app.kubernetes.io/component: ui
{{- end }}

{{/*
PostgreSQL labels
*/}}
{{- define "streamspace.postgresql.labels" -}}
{{ include "streamspace.labels" . }}
app.kubernetes.io/component: database
{{- end }}

{{/*
Create the name of the controller service account to use
*/}}
{{- define "streamspace.controller.serviceAccountName" -}}
{{- if .Values.controller.serviceAccount.create }}
{{- default (printf "%s-controller" (include "streamspace.fullname" .)) .Values.controller.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.controller.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the API service account to use
*/}}
{{- define "streamspace.api.serviceAccountName" -}}
{{- if .Values.api.serviceAccount.create }}
{{- default (printf "%s-api" (include "streamspace.fullname" .)) .Values.api.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.api.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Get the PostgreSQL host
*/}}
{{- define "streamspace.postgresql.host" -}}
{{- if .Values.postgresql.external.enabled }}
{{- .Values.postgresql.external.host }}
{{- else }}
{{- printf "%s-postgres" (include "streamspace.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Get the PostgreSQL port
*/}}
{{- define "streamspace.postgresql.port" -}}
{{- if .Values.postgresql.external.enabled }}
{{- .Values.postgresql.external.port }}
{{- else }}
{{- 5432 }}
{{- end }}
{{- end }}

{{/*
Get the PostgreSQL database name
*/}}
{{- define "streamspace.postgresql.database" -}}
{{- if .Values.postgresql.external.enabled }}
{{- .Values.postgresql.external.database }}
{{- else }}
{{- .Values.postgresql.auth.database }}
{{- end }}
{{- end }}

{{/*
Get the PostgreSQL username
*/}}
{{- define "streamspace.postgresql.username" -}}
{{- if .Values.postgresql.external.enabled }}
{{- .Values.postgresql.external.username }}
{{- else }}
{{- .Values.postgresql.auth.username }}
{{- end }}
{{- end }}

{{/*
Get the PostgreSQL secret name
*/}}
{{- define "streamspace.postgresql.secretName" -}}
{{- if .Values.secrets.existingSecret }}
{{- .Values.secrets.existingSecret }}
{{- else if .Values.postgresql.auth.existingSecret }}
{{- .Values.postgresql.auth.existingSecret }}
{{- else }}
{{- printf "%s-secrets" (include "streamspace.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Get the PostgreSQL secret key for password
*/}}
{{- define "streamspace.postgresql.secretKey" -}}
{{- if .Values.secrets.existingSecret }}
{{- .Values.secrets.existingSecretKeys.postgresPassword }}
{{- else if .Values.postgresql.auth.existingSecret }}
{{- .Values.postgresql.auth.secretKeys.adminPasswordKey }}
{{- else }}
{{- "postgres-password" }}
{{- end }}
{{- end }}

{{/*
Image name for controller
*/}}
{{- define "streamspace.controller.image" -}}
{{- $registry := .Values.global.imageRegistry | default .Values.controller.image.registry }}
{{- $repository := .Values.controller.image.repository }}
{{- $tag := .Values.controller.image.tag | default .Chart.AppVersion }}
{{- if $registry }}
{{- printf "%s/%s:%s" $registry $repository $tag }}
{{- else }}
{{- printf "%s:%s" $repository $tag }}
{{- end }}
{{- end }}

{{/*
Image name for API
*/}}
{{- define "streamspace.api.image" -}}
{{- $registry := .Values.global.imageRegistry | default .Values.api.image.registry }}
{{- $repository := .Values.api.image.repository }}
{{- $tag := .Values.api.image.tag | default .Chart.AppVersion }}
{{- if $registry }}
{{- printf "%s/%s:%s" $registry $repository $tag }}
{{- else }}
{{- printf "%s:%s" $repository $tag }}
{{- end }}
{{- end }}

{{/*
Image name for UI
*/}}
{{- define "streamspace.ui.image" -}}
{{- $registry := .Values.global.imageRegistry | default .Values.ui.image.registry }}
{{- $repository := .Values.ui.image.repository }}
{{- $tag := .Values.ui.image.tag | default .Chart.AppVersion }}
{{- if $registry }}
{{- printf "%s/%s:%s" $registry $repository $tag }}
{{- else }}
{{- printf "%s:%s" $repository $tag }}
{{- end }}
{{- end }}

{{/*
Image name for PostgreSQL
*/}}
{{- define "streamspace.postgresql.image" -}}
{{- $registry := .Values.global.imageRegistry | default .Values.postgresql.internal.image.registry }}
{{- $repository := .Values.postgresql.internal.image.repository }}
{{- $tag := .Values.postgresql.internal.image.tag }}
{{- if $registry }}
{{- printf "%s/%s:%s" $registry $repository $tag }}
{{- else }}
{{- printf "%s:%s" $repository $tag }}
{{- end }}
{{- end }}

{{/*
Storage class for PostgreSQL
*/}}
{{- define "streamspace.postgresql.storageClass" -}}
{{- .Values.global.storageClass | default .Values.postgresql.internal.persistence.storageClass }}
{{- end }}

{{/*
Storage class for user home directories
*/}}
{{- define "streamspace.sessionDefaults.storageClass" -}}
{{- .Values.global.storageClass | default .Values.sessionDefaults.persistentHome.storageClass }}
{{- end }}
