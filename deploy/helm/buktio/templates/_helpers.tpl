{{- define "buktio.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "buktio.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name (include "buktio.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "buktio.labels" -}}
app.kubernetes.io/name: {{ include "buktio.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version }}
{{- end -}}

{{- define "buktio.selectorLabels" -}}
app.kubernetes.io/name: {{ include "buktio.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "buktio.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "buktio.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{/* The image tag for a component, defaulting to the chart appVersion. */}}
{{- define "buktio.imageTag" -}}
{{- default .Chart.AppVersion .Values.image.tag -}}
{{- end -}}

{{/* DATABASE_URL: external if set, else the bundled Postgres service. When RLS is
on and an appUser is set, connect as that non-superuser role. */}}
{{- define "buktio.databaseUrl" -}}
{{- if .Values.externalDatabase.url -}}
{{- .Values.externalDatabase.url -}}
{{- else -}}
{{- $user := .Values.postgres.user -}}
{{- if and .Values.api.rls .Values.postgres.appUser -}}{{- $user = .Values.postgres.appUser -}}{{- end -}}
{{- printf "postgres://%s:$(POSTGRES_PASSWORD)@%s-postgres:5432/%s?sslmode=disable" $user (include "buktio.fullname" .) .Values.postgres.database -}}
{{- end -}}
{{- end -}}
