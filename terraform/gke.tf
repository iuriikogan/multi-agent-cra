resource "google_container_cluster" "primary" {
  name     = var.cluster_name
  location = var.region

  # Using Autopilot for "Agent Engine" optimization (automatic scaling/management)
  enable_autopilot = true

  network    = google_compute_network.vpc.name
  subnetwork = google_compute_subnetwork.subnet.name

  ip_allocation_policy {
    cluster_secondary_range_name  = "pods"
    services_secondary_range_name = "services"
  }

  # Enable Gateway API
  gateway_api_config {
    channel = "CHANNEL_STANDARD"
  }

  # Enable Secret Manager CSI Driver
  secret_manager_config {
    enabled = true
  }

  resource_labels = {
    env  = "production"
    team = "agent-cra"
  }

  private_cluster_config {
    enable_private_nodes    = true
    enable_private_endpoint = true
    master_ipv4_cidr_block  = var.master_ipv4_cidr_block
  }

  master_authorized_networks_config {
    cidr_blocks {
      cidr_block   = "10.10.0.0/28" # Allow Bastion Subnet
      display_name = "Bastion Subnet"
    }
  }

  master_auth {
    client_certificate_config {
      issue_client_certificate = false
    }
  }

  # Workload Identity is enabled by default on Autopilot
  deletion_protection = false # For easier cleanup in this demo
}
