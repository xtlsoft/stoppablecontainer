{{/*
Expand the name of the chart.
*/}}
{{- define "stoppablecontainer.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "stoppablecontainer.fullname" -}}
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
{{- define "stoppablecontainer.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "stoppablecontainer.labels" -}}
helm.sh/chart: {{ include "stoppablecontainer.chart" . }}
{{ include "stoppablecontainer.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "stoppablecontainer.selectorLabels" -}}
app.kubernetes.io/name: {{ include "stoppablecontainer.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "stoppablecontainer.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "stoppablecontainer.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Controller image
*/}}
{{- define "stoppablecontainer.controllerImage" -}}
{{- $tag := default .Chart.AppVersion .Values.controller.image.tag -}}
{{- printf "%s:%s" .Values.controller.image.repository $tag }}
{{- end }}

{{/*
Mount-helper image
*/}}
{{- define "stoppablecontainer.mountHelperImage" -}}
{{- $tag := default .Chart.AppVersion .Values.mountHelper.image.tag -}}
{{- printf "%s:%s" .Values.mountHelper.image.repository $tag }}
{{- end }}

{{/*
Exec-wrapper image
*/}}
{{- define "stoppablecontainer.execWrapperImage" -}}
{{- $tag := default .Chart.AppVersion .Values.execWrapper.image.tag -}}
{{- printf "%s:%s" .Values.execWrapper.image.repository $tag }}
{{- end }}
