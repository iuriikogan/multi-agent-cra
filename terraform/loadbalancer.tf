# loadbalancer.tf - Global External Application Load Balancer with Cloud Armor

resource "google_compute_global_address" "default" {
  name = "compliance-dashboard-ip"
}

resource "google_compute_region_network_endpoint_group" "serverless_neg" {
  name                  = "compliance-server-neg"
  network_endpoint_type = "SERVERLESS"
  region                = var.region
  cloud_run {
    service = google_cloud_run_v2_service.server.name
  }
}

resource "google_compute_backend_service" "default" {
  name                  = "compliance-backend"
  protocol              = "HTTPS"
  port_name             = "http"
  load_balancing_scheme = "EXTERNAL_MANAGED"
  security_policy       = google_compute_security_policy.policy.name

  backend {
    group = google_compute_region_network_endpoint_group.serverless_neg.id
  }
}

resource "google_compute_url_map" "default" {
  name            = "compliance-url-map"
  default_service = google_compute_backend_service.default.id
}

resource "google_compute_target_http_proxy" "default" {
  name    = "compliance-http-proxy"
  url_map = google_compute_url_map.default.id
}

resource "google_compute_global_forwarding_rule" "default" {
  name                  = "compliance-frontend-rule"
  target                = google_compute_target_http_proxy.default.id
  port_range            = "80"
  ip_address            = google_compute_global_address.default.address
  load_balancing_scheme = "EXTERNAL_MANAGED"
}

resource "google_compute_security_policy" "policy" {
  name        = "compliance-security-policy"
  description = "Basic security policy for Compliance Dashboard"

  rule {
    action   = "deny(403)"
    priority = "2147483647"
    match {
      versioned_expr = "SRC_IPS_V1"
      config {
        src_ip_ranges = ["*"]
      }
    }
    description = "Default deny all"
  }

  rule {
    action   = "allow"
    priority = "1000"
    match {
      versioned_expr = "SRC_IPS_V1"
      config {
        src_ip_ranges = ["${chomp(data.http.myip.response_body)}/32"]
      }
    }
    description = "Allow my IP"
  }
}

data "http" "myip" {
  url = "https://ipv4.icanhazip.com"
}
