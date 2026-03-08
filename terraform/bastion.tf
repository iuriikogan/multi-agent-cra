# Bastion Host Subnet
resource "google_compute_subnetwork" "bastion_subnet" {
  name          = "${var.cluster_name}-bastion-subnet"
  region        = var.region
  network       = google_compute_network.vpc.name
  ip_cidr_range = "10.10.0.0/28"

  log_config {
    aggregation_interval = "INTERVAL_10_MIN"
    flow_sampling        = 0.5
    metadata             = "INCLUDE_ALL_METADATA"
  }
}

# Cloud NAT (Required for Private Cluster nodes and Bastion to access internet)
resource "google_compute_router" "router" {
  name    = "${var.cluster_name}-router"
  region  = var.region
  network = google_compute_network.vpc.name
}

resource "google_compute_router_nat" "nat" {
  name                               = "${var.cluster_name}-nat"
  router                             = google_compute_router.router.name
  region                             = var.region
  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "ALL_SUBNETWORKS_ALL_IP_RANGES"
}

# Firewall rule to allow IAP SSH to Bastion
resource "google_compute_firewall" "allow_iap_ssh" {
  name    = "allow-iap-ssh"
  network = google_compute_network.vpc.name

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }

  source_ranges = ["35.235.240.0/20"] # Google IAP IP range
  target_tags   = ["bastion"]
}

# Dedicated Service Account for Bastion
resource "google_service_account" "bastion_sa" {
  account_id   = "bastion-sa"
  display_name = "Bastion Host Service Account"
}

# Bastion Host Instance
resource "google_compute_instance" "bastion" {
  name         = "gke-bastion"
  machine_type = "e2-micro"
  zone         = "${var.region}-a"

  tags = ["bastion"]

  boot_disk {
    initialize_params {
      image = "debian-cloud/debian-11"
    }
  }

  network_interface {
    subnetwork = google_compute_subnetwork.bastion_subnet.name
    # No access_config block = No public IP
  }

  service_account {
    email  = google_service_account.bastion_sa.email
    scopes = ["cloud-platform"]
  }

  metadata = {
    enable-oslogin = "TRUE"
    block-project-ssh-keys = "TRUE"
  }

  deletion_protection = true

  shielded_instance_config {
    enable_secure_boot          = true
    enable_vtpm                 = true
    enable_integrity_monitoring = true
  }
}
