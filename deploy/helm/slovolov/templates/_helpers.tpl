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

{{- define "slovolov.anthropicSecretName" -}}
{{- if .Values.anthropic.existingSecret -}}
{{ .Values.anthropic.existingSecret }}
{{- else -}}
{{ include "slovolov.fullname" . }}-anthropic
{{- end -}}
{{- end -}}

{{- define "slovolov.anthropicSecretKey" -}}
{{- if .Values.anthropic.existingSecret -}}
{{ .Values.anthropic.existingSecretKey }}
{{- else -}}
anthropic-api-key
{{- end -}}
{{- end -}}

{{/*
Env block shared by the server Deployment and the daily CronJob: cache
address, Claude credentials, and the puzzle timezone.
*/}}
{{- define "slovolov.commonEnv" -}}
- name: REDIS_ADDR
  value: {{ include "slovolov.dragonflyAddress" . | quote }}
- name: PUZZLE_TIMEZONE
  value: {{ .Values.puzzle.timezone | quote }}
- name: ANTHROPIC_API_KEY
  valueFrom:
    secretKeyRef:
      name: {{ include "slovolov.anthropicSecretName" . }}
      key: {{ include "slovolov.anthropicSecretKey" . }}
{{- if .Values.anthropic.model }}
- name: ANTHROPIC_MODEL
  value: {{ .Values.anthropic.model | quote }}
{{- end }}
{{- if .Values.anthropic.bulkModel }}
- name: ANTHROPIC_BULK_MODEL
  value: {{ .Values.anthropic.bulkModel | quote }}
{{- end }}
{{- if .Values.anthropic.guessModel }}
- name: ANTHROPIC_GUESS_MODEL
  value: {{ .Values.anthropic.guessModel | quote }}
{{- end }}
{{- end -}}
