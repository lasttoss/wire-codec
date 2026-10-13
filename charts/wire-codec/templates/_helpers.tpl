{{- define "wire-codec.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "wire-codec.fullname" -}}
{{- printf "%s" (include "wire-codec.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "wire-codec.labels" -}}
app.kubernetes.io/name: {{ include "wire-codec.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version }}
{{- end -}}
