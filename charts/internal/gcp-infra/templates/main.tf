provider "google" {
  credentials = "${var.SERVICEACCOUNT}"
  project     = "{{ required "google.project is required" .Values.google.project }}"
  region      = "{{ required "google.region is required" .Values.google.region }}"
}

//=====================================================================
//= Service Account
//=====================================================================

resource "google_service_account" "serviceaccount" {
  account_id   = "{{ required "clusterName is required" .Values.clusterName }}"
  display_name = "{{ required "clusterName is required" .Values.clusterName }}"
}

//=====================================================================
//= Networks
//=====================================================================

{{ if .Values.create.vpc -}}
resource "google_compute_network" "network" {
  name                    = "{{ required "clusterName is required" .Values.clusterName }}"
  auto_create_subnetworks = "false"
}
{{- end}}

resource "google_compute_subnetwork" "subnetwork-nodes" {
  name          = "{{ required "clusterName is required" .Values.clusterName }}-nodes"
  ip_cidr_range = "{{ required "networks.workers is required" .Values.networks.workers }}"
  network       = "{{ required "vpc.name is required" .Values.vpc.name }}"
  region        = "{{ required "google.region is required" .Values.google.region }}"
{{- if .Values.networks.flowLogs }}
  log_config {
    {{ if .Values.networks.flowLogs.aggregationInterval }}aggregation_interval = "{{ .Values.networks.flowLogs.aggregationInterval }}"{{ end }}
    {{ if .Values.networks.flowLogs.flowSampling }}flow_sampling        = "{{ .Values.networks.flowLogs.flowSampling }}"{{ end }}
    {{ if .Values.networks.flowLogs.metadata }}metadata             = "{{ .Values.networks.flowLogs.metadata }}"{{ end }}
  }
{{- end }}
}

{{ if .Values.create.cloudRouter -}}
resource "google_compute_router" "router"{
  name    = "{{ required "clusterName is required" .Values.clusterName }}-cloud-router"
  region  = "{{ required "google.region is required" .Values.google.region }}"
  network = "{{ required "vpc.name is required" .Values.vpc.name }}"
}
{{- end }}

{{ if or  .Values.create.cloudRouter .Values.vpc.cloudRouter -}}
resource "google_compute_router_nat" "nat" {
  name                               = "{{ required "clusterName is required" .Values.clusterName }}-cloud-nat"
  {{  if .Values.vpc.cloudRouter -}}
  router                             = "{{ required "vpc.cloudRouter.name is required" .Values.vpc.cloudRouter.name }}"
  {{ else -}}
  router =  "${google_compute_router.router.name}"
  {{ end -}}
  region                             = "{{ required "google.region is required" .Values.google.region }}"
  nat_ip_allocate_option             = {{ if .Values.networks.cloudNAT.natIPNames }}"MANUAL_ONLY"{{ else }}"AUTO_ONLY"{{ end }}
  {{ if .Values.networks.cloudNAT.natIPNames -}}
  nat_ips                = [{{range $i, $name := .Values.networks.cloudNAT.natIPNames}}{{if $i}},{{end}}"${data.google_compute_address.{{$name}}.self_link}"{{end}}]
  {{- end }}

  source_subnetwork_ip_ranges_to_nat = "LIST_OF_SUBNETWORKS"
  subnetwork {
    name                    =  "${google_compute_subnetwork.subnetwork-nodes.self_link}"
    source_ip_ranges_to_nat = ["ALL_IP_RANGES"]
  }
  min_ports_per_vm = "{{ required "networks.cloudNAT.minPortsPerVM is required" .Values.networks.cloudNAT.minPortsPerVM }}"

  log_config {
    enable = true
    filter = "ERRORS_ONLY"
  }
}
{{- end}}

{{ if .Values.networks.cloudNAT.natIPNames -}}
{{range $index, $natIP := .Values.networks.cloudNAT.natIPNames}}
data "google_compute_address" "{{ $natIP }}" {
  name = "{{ $natIP }}"
}
{{end}}
{{- end }}

{{ if .Values.networks.internal -}}
resource "google_compute_subnetwork" "subnetwork-internal" {
  name          = "{{ required "clusterName is required" .Values.clusterName }}-internal"
  ip_cidr_range = "{{ required "networks.internal is required" .Values.networks.internal }}"
  network       = "{{ required "vpc.name is required" .Values.vpc.name }}"
  region        = "{{ required "google.region is required" .Values.google.region }}"
}
{{- end}}
//=====================================================================
//= Firewall
//=====================================================================

// Allow traffic within internal network range.
resource "google_compute_firewall" "rule-allow-internal-access" {
  name          = "{{ required "clusterName is required" .Values.clusterName }}-allow-internal-access"
  network       = "{{ required "vpc.name is required" .Values.vpc.name }}"
  source_ranges = ["10.0.0.0/8"]

  allow {
    protocol = "icmp"
  }

  allow {
    protocol = "ipip"
  }

  allow {
    protocol = "tcp"
    ports    = ["1-65535"]
  }

  allow {
    protocol = "udp"
    ports    = ["1-65535"]
  }
}

resource "google_compute_firewall" "rule-allow-external-access" {
  name          = "{{ required "clusterName is required" .Values.clusterName }}-allow-external-access"
  network       = "{{ required "vpc.name is required" .Values.vpc.name }}"
  source_ranges = ["0.0.0.0/0"]

  allow {
    protocol = "tcp"
    ports    = ["80", "443"] // Allow ingress
  }
}

// Required to allow Google to perform health checks on our instances.
// https://cloud.google.com/compute/docs/load-balancing/internal/
// https://cloud.google.com/compute/docs/load-balancing/network/
resource "google_compute_firewall" "rule-allow-health-checks" {
  name          = "{{ required "clusterName is required" .Values.clusterName }}-allow-health-checks"
  network       = "{{ required "vpc.name is required" .Values.vpc.name }}"
  source_ranges = [
    "35.191.0.0/16",
    "209.85.204.0/22",
    "209.85.152.0/22",
    "130.211.0.0/22",
  ]

  allow {
    protocol = "tcp"
    ports    = ["30000-32767"]
  }

  allow {
    protocol = "udp"
    ports    = ["30000-32767"]
  }
}

// We have introduced new output variables. However, they are not applied for
// existing clusters as Terraform won't detect a diff when we run `terraform plan`.
// Workaround: Providing a null-resource for letting Terraform think that there are
// differences, enabling the Gardener to start an actual `terraform apply` job.
resource "null_resource" "outputs" {
  triggers = {
    recompute = "outputs"
  }
}

//=====================================================================
//= Output variables
//=====================================================================

output "{{ .Values.outputKeys.vpcName }}" {
  value = "{{ required "vpc.name is required" .Values.vpc.name }}"
}

{{ if or  .Values.create.cloudRouter .Values.vpc.cloudRouter -}}
output "{{ .Values.outputKeys.cloudRouter }}" {
  {{ if .Values.create.cloudRouter -}}
  value = "${google_compute_router.router.name}"
  {{ else -}}
  value = "{{ .Values.vpc.cloudRouter.name }}"
  {{ end -}}
}

output "{{ .Values.outputKeys.cloudNAT }}" {
  value = "${google_compute_router_nat.nat.name}"
}
{{- end }}

{{ if .Values.networks.cloudNAT.natIPNames -}}
output "{{ .Values.outputKeys.natIPs }}" {
    value = "{{range $i, $name := .Values.networks.cloudNAT.natIPNames}}{{if $i}},{{end}}${data.google_compute_address.{{$name}}.address}{{end}}"
}
{{- end }}

output "{{ .Values.outputKeys.serviceAccountEmail }}" {
  value = "${google_service_account.serviceaccount.email}"
}

output "{{ .Values.outputKeys.subnetNodes }}" {
  value = "${google_compute_subnetwork.subnetwork-nodes.name}"
}
{{ if .Values.networks.internal -}}
output "{{ .Values.outputKeys.subnetInternal }}" {
  value = "${google_compute_subnetwork.subnetwork-internal.name}"
}
{{- end}}
