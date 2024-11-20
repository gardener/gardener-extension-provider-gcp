provider "google" {
  credentials = var.SERVICEACCOUNT
  project     = "{{ .google.project }}"
  region      = "{{ .google.region }}"
}

//=====================================================================
//= Service Account
//=====================================================================

{{ if .create.serviceAccount -}}
resource "google_service_account" "serviceaccount" {
  account_id   = "{{ .clusterName }}"
  display_name = "{{ .clusterName }}"
}
{{- end }}

//=====================================================================
//= Networks
//=====================================================================

{{ if .create.vpc -}}
resource "google_compute_network" "network" {
  name                    = "{{ .clusterName }}"
  auto_create_subnetworks = "false"
{{ if .networks.dualStack }}
  enable_ula_internal_ipv6 = true  # Enable ULA for internal IPv6
  routing_mode             = "GLOBAL" # Required for dual-stack networks
{{ end }}
  timeouts {
    create = "5m"
    update = "5m"
    delete = "5m"
  }
}
{{- end }}

resource "google_compute_subnetwork" "subnetwork-nodes" {
  name          = "{{ .clusterName }}-nodes"
  ip_cidr_range = "{{ .networks.workers }}"
  network       = {{ .vpc.name }}
  region        = "{{ .google.region }}"
{{ if .networks.dualStack }}
  ipv6_access_type    = "EXTERNAL"  # or "INTERNAL" based on your needs
{{ if .networks.ipv6CidrRange }}
  ipv6_cidr_range = "{{ .networks.ipv6CidrRange }}"
{{ end }}
  stack_type          = "IPV4_IPV6"  # Enable dual-stack
{{ end }}

{{- if .networks.flowLogs }}
  log_config {
    {{ if .networks.flowLogs.aggregationInterval }}aggregation_interval = "{{ .networks.flowLogs.aggregationInterval }}"{{ end }}
    {{ if .networks.flowLogs.flowSampling }}flow_sampling        = "{{ .networks.flowLogs.flowSampling }}"{{ end }}
    {{ if .networks.flowLogs.metadata }}metadata             = "{{ .networks.flowLogs.metadata }}"{{ end }}
  }
{{- end }}

  timeouts {
    create = "5m"
    update = "5m"
    delete = "5m"
  }
}

{{ if .create.cloudRouter -}}
resource "google_compute_router" "router" {
  name    = "{{ .clusterName }}-cloud-router"
  region  = "{{ .google.region }}"
  network = {{ .vpc.name }}

  timeouts {
    create = "5m"
    update = "5m"
    delete = "5m"
  }
}
{{- end }}

{{ if or  .create.cloudRouter .vpc.cloudRouter -}}
resource "google_compute_router_nat" "nat" {
  name                               = "{{ .clusterName }}-cloud-nat"
  {{  if .vpc.cloudRouter -}}
  router                             = "{{ .vpc.cloudRouter.name }}"
  {{ else -}}
  router = google_compute_router.router.name
  {{ end -}}
  region                             = "{{ .google.region }}"
  nat_ip_allocate_option             = {{ if .networks.cloudNAT.natIPNames }}"MANUAL_ONLY"{{ else }}"AUTO_ONLY"{{ end }}
  {{ if .networks.cloudNAT.natIPNames -}}
  nat_ips                = [{{range $i, $name := .networks.cloudNAT.natIPNames}}{{if $i}},{{end}}data.google_compute_address.{{$name}}.self_link{{end}}]
  {{- end }}

  source_subnetwork_ip_ranges_to_nat = "LIST_OF_SUBNETWORKS"
  subnetwork {
    name                    = google_compute_subnetwork.subnetwork-nodes.self_link
    source_ip_ranges_to_nat = ["ALL_IP_RANGES"]
  }

  enable_dynamic_port_allocation = {{ .networks.cloudNAT.enableDynamicPortAllocation }}
  enable_endpoint_independent_mapping = {{ .networks.cloudNAT.enableEndpointIndependentMapping }}
  min_ports_per_vm = "{{ .networks.cloudNAT.minPortsPerVM }}"
  {{  if .networks.cloudNAT.maxPortsPerVM -}}
  max_ports_per_vm = "{{ .networks.cloudNAT.maxPortsPerVM }}"
  {{- end }}
  icmp_idle_timeout_sec = "{{ .networks.cloudNAT.icmpIdleTimeoutSec }}"
  tcp_established_idle_timeout_sec = "{{ .networks.cloudNAT.tcpEstablishedIdleTimeoutSec }}"
  tcp_time_wait_timeout_sec = "{{ .networks.cloudNAT.tcpTimeWaitTimeoutSec }}"
  tcp_transitory_idle_timeout_sec = "{{ .networks.cloudNAT.tcpTransitoryIdleTimeoutSec }}"
  udp_idle_timeout_sec = "{{ .networks.cloudNAT.udpIdleTimeoutSec }}"

  log_config {
    enable = true
    filter = "ERRORS_ONLY"
  }

  timeouts {
    create = "5m"
    update = "5m"
    delete = "5m"
  }
}
{{- end}}

{{ if .networks.cloudNAT.natIPNames -}}
{{range $index, $natIP := .networks.cloudNAT.natIPNames}}
data "google_compute_address" "{{ $natIP }}" {
  name = "{{ $natIP }}"
}
{{end}}
{{- end }}

{{ if .networks.internal -}}
resource "google_compute_subnetwork" "subnetwork-internal" {
  name          = "{{ .clusterName }}-internal"
  ip_cidr_range = "{{ .networks.internal }}"
  network       = {{ .vpc.name }}
  region        = "{{ .google.region }}"
{{ if .networks.dualStack }}
    ipv6_access_type    = "EXTERNAL"  # or "INTERNAL"
    stack_type          = "IPV4_IPV6"
    
{{ if .networks.ipv6CidrRange }}
    ipv6_cidr_range = "{{ .networks.ipv6CidrRange }}"
{{ end }}
{{ end }}

  timeouts {
    create = "5m"
    update = "5m"
    delete = "5m"
  }
}
{{- end}}

//=====================================================================
//= Firewall
//=====================================================================

// Allow traffic within internal network range.
resource "google_compute_firewall" "rule-allow-internal-access" {
  name          = "{{ .clusterName }}-allow-internal-access"
  network       = {{ .vpc.name }}
  {{ if .networks.internal -}}
  source_ranges = ["{{ .networks.workers }}", "{{ .networks.internal }}", "{{ .podCIDR }}"]
  {{ else -}}
  source_ranges = ["{{ .networks.workers }}", "{{ .podCIDR }}"]
  {{ end -}}

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

  timeouts {
    create = "5m"
    update = "5m"
    delete = "5m"
  }
}

// Required to allow Google to perform health checks on our instances.
// https://cloud.google.com/compute/docs/load-balancing/internal/
// https://cloud.google.com/compute/docs/load-balancing/network/
resource "google_compute_firewall" "rule-allow-health-checks" {
  name          = "{{ .clusterName }}-allow-health-checks"
  network       = {{ .vpc.name }}
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

  timeouts {
    create = "5m"
    update = "5m"
    delete = "5m"
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

output "{{ .outputKeys.vpcName }}" {
  value = {{ .vpc.name }}
}

{{ if or  .create.cloudRouter .vpc.cloudRouter -}}
output "{{ .outputKeys.cloudRouter }}" {
  {{ if .create.cloudRouter -}}
  value = google_compute_router.router.name
  {{ else -}}
  value = "{{ .vpc.cloudRouter.name }}"
  {{- end }}
}

output "{{ .outputKeys.cloudNAT }}" {
  value = google_compute_router_nat.nat.name
}
{{- end }}

{{ if .networks.cloudNAT.natIPNames -}}
output "{{ .outputKeys.natIPs }}" {
  value = "{{range $i, $name := .networks.cloudNAT.natIPNames}}{{if $i}},{{end}}${data.google_compute_address.{{$name}}.address}{{end}}"
}
{{- end }}

{{ if .create.serviceAccount -}}
output "{{ .outputKeys.serviceAccountEmail }}" {
  value = google_service_account.serviceaccount.email
}
{{- end }}

output "{{ .outputKeys.subnetNodes }}" {
  value = google_compute_subnetwork.subnetwork-nodes.name
}
{{ if .networks.internal -}}
output "{{ .outputKeys.subnetInternal }}" {
  value = google_compute_subnetwork.subnetwork-internal.name
}
{{- end}}
