{{- define "slovolov.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "slovolov.fullname" -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "slovolov.labels" -}}
app.kubernetes.io/name: {{ include "slovolov.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end -}}

{{- define "slovolov.selectorLabels" -}}
app.kubernetes.io/name: {{ include "slovolov.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "slovolov.image" -}}
{{ .Values.image.repository }}:{{ default .Chart.AppVersion .Values.image.tag }}
{{- end -}}

{{- define "slovolov.dragonflyAddress" -}}
{{- if .Values.dragonfly.addressOverride -}}
{{ .Values.dragonfly.addressOverride }}
{{- else -}}
{{ .Values.dragonfly.name }}.{{ .Release.Namespace }}.svc.cluster.local:6379
{{- end -}}
{{- end -}}

{{- define "slovolov.openaiSecretName" -}}
{{- if .Values.openai.existingSecret -}}
{{ .Values.openai.existingSecret }}
{{- else -}}
{{ include "slovolov.fullname" . }}-openai
{{- end -}}
{{- end -}}

{{- define "slovolov.openaiSecretKey" -}}
{{- if .Values.openai.existingSecret -}}
{{ .Values.openai.existingSecretKey }}
{{- else -}}
openai-api-key
{{- end -}}
{{- end -}}

{{- define "slovolov.cohereSecretName" -}}
{{- if .Values.cohere.existingSecret -}}
{{ .Values.cohere.existingSecret }}
{{- else -}}
{{ include "slovolov.fullname" . }}-cohere
{{- end -}}
{{- end -}}

{{- define "slovolov.cohereSecretKey" -}}
{{- if .Values.cohere.existingSecret -}}
{{ .Values.cohere.existingSecretKey }}
{{- else -}}
cohere-api-key
{{- end -}}
{{- end -}}

{{/*
Render the embedding provider env block. Emits EMBED_PROVIDER plus the
provider-specific key + optional model/input-type overrides. Skips
providers that don't have credentials configured so a Deployment
manifest doesn't reference missing secrets.
*/}}
{{- define "slovolov.embedEnv" -}}
- name: EMBED_PROVIDER
  value: {{ .Values.embedProvider | quote }}
{{- if or .Values.openai.apiKey .Values.openai.existingSecret }}
- name: OPENAI_API_KEY
  valueFrom:
    secretKeyRef:
      name: {{ include "slovolov.openaiSecretName" . }}
      key: {{ include "slovolov.openaiSecretKey" . }}
{{- end }}
{{- if or .Values.cohere.apiKey .Values.cohere.existingSecret }}
- name: COHERE_API_KEY
  valueFrom:
    secretKeyRef:
      name: {{ include "slovolov.cohereSecretName" . }}
      key: {{ include "slovolov.cohereSecretKey" . }}
{{- if .Values.cohere.model }}
- name: COHERE_MODEL
  value: {{ .Values.cohere.model | quote }}
{{- end }}
{{- if .Values.cohere.inputType }}
- name: COHERE_INPUT_TYPE
  value: {{ .Values.cohere.inputType | quote }}
{{- end }}
{{- end }}
{{- end -}}
