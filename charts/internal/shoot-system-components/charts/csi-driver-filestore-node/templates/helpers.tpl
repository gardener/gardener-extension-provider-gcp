{{- define "csi-driver-filestore-node.extensionsGroup" -}}
extensions.gardener.cloud
{{- end -}}

{{- define "csi-driver-filestore-node.name" -}}
provider-gcp
{{- end -}}

{{- define "csi-driver-filestore-node.provisioner" -}}
filestore.csi.storage.gke.io
{{- end -}}

{{- define "csi-driver-filestore-node.storageversion" -}}
storage.k8s.io/v1
{{- end -}}
