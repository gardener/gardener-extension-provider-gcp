{{- define "csi-driver-node.extensionsGroup" -}}
extensions.gardener.cloud
{{- end -}}

{{- define "csi-driver-node.name" -}}
provider-gcp
{{- end -}}

{{- define "csi-driver-node.provisioner" -}}
pd.csi.storage.gke.io
{{- end -}}

{{- define "csi-driver-node.storageversion" -}}
{{- if semverCompare "<= 1.18.x" .Values.kubernetesVersion -}}
storage.k8s.io/v1beta1
{{- else -}}
storage.k8s.io/v1
{{- end -}}
{{- end -}}
