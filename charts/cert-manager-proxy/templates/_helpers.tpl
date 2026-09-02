{{- define "cert-manager-proxy.fullname" -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "cert-manager-proxy.labels" -}}
app.kubernetes.io/name: {{ include "cert-manager-proxy.fullname" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "cert-manager-proxy.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "cert-manager-proxy.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- required "serviceAccount.name is required when serviceAccount.create is false" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- define "cert-manager-proxy.namespace" -}}
{{- default .Release.Namespace .Values.namespace -}}
{{- end -}}

{{- define "cert-manager-proxy.authSecretName" -}}
{{- if .Values.auth.existingSecretName -}}
{{- .Values.auth.existingSecretName -}}
{{- else -}}
{{- printf "%s-auth" (include "cert-manager-proxy.fullname" .) -}}
{{- end -}}
{{- end -}}
