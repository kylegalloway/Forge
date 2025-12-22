{{/*
Expand the name of the chart.
*/}}
{{- define "forge.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "forge.fullname" -}}
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
{{- define "forge.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "forge.labels" -}}
helm.sh/chart: {{ include "forge.chart" . }}
{{ include "forge.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "forge.selectorLabels" -}}
app.kubernetes.io/name: {{ include "forge.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Controller labels
*/}}
{{- define "forge.controller.labels" -}}
{{ include "forge.labels" . }}
app: forge-controller
app.kubernetes.io/component: controller
app.kubernetes.io/part-of: forge
{{- end }}

{{/*
Controller selector labels
*/}}
{{- define "forge.controller.selectorLabels" -}}
app: forge-controller
{{ include "forge.selectorLabels" . }}
{{- end }}

{{/*
Webhook labels
*/}}
{{- define "forge.webhook.labels" -}}
{{ include "forge.labels" . }}
app: forge-webhook
app.kubernetes.io/component: webhook
app.kubernetes.io/part-of: forge
{{- end }}

{{/*
Webhook selector labels
*/}}
{{- define "forge.webhook.selectorLabels" -}}
app: forge-webhook
{{ include "forge.selectorLabels" . }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "forge.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default "forge-controller" .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Namespace
*/}}
{{- define "forge.namespace" -}}
{{- default "forge-system" .Values.global.namespace }}
{{- end }}
