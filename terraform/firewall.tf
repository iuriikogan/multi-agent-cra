# Firewall Rules for Egress Control

# Deny all egress traffic by default (Priority 65535 is lowest)
resource "google_compute_firewall" "deny_all_egress" {
  name               = "deny-all-egress"
  network            = google_compute_network.vpc.name
  direction          = "EGRESS"
  destination_ranges = ["0.0.0.0/0"]
  priority           = 65535

  deny {
    protocol = "all"
  }
}

# Allow egress to internal VPC ranges (Priority 1000 is higher)
resource "google_compute_firewall" "allow_egress_internal" {
  name               = "allow-egress-internal"
  network            = google_compute_network.vpc.name
  direction          = "EGRESS"
  destination_ranges = ["10.0.0.0/8"] # Covers VPC subnets
  priority           = 1000

  allow {
    protocol = "all"
  }
}

# Allow egress to GKE Master (Priority 1000)
resource "google_compute_firewall" "allow_egress_gke_master" {
  name               = "allow-egress-gke-master"
  network            = google_compute_network.vpc.name
  direction          = "EGRESS"
  destination_ranges = [var.master_ipv4_cidr_block]
  priority           = 1000

  allow {
    protocol = "tcp"
    ports    = ["443", "10250"] # Kube API, Kubelet
  }
}

# Allow essential external services (HTTPS, DNS, NTP) (Priority 1000)
resource "google_compute_firewall" "allow_egress_external" {
  name               = "allow-egress-external"
  network            = google_compute_network.vpc.name
  direction          = "EGRESS"
  destination_ranges = ["0.0.0.0/0"]
  priority           = 2000 # Lower priority than internal/master specific rules, but higher than deny-all

  allow {
    protocol = "tcp"
    ports    = ["80", "443", "53"]
  }

  allow {
    protocol = "udp"
    ports    = ["53", "123"]
  }
}
