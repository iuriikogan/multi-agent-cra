# Package vpc defines the network topology and serverless connectivity for the system.

resource "google_compute_network" "vpc" {
  name                    = "compliance-vpc"
  auto_create_subnetworks = false # Custom subnetting for better control
}

resource "google_compute_subnetwork" "default" {
  name          = "compliance-subnet"
  ip_cidr_range = "10.0.0.0/24"
  network       = google_compute_network.vpc.id
  region        = var.region

  log_config {
    aggregation_interval = "INTERVAL_10_MIN"
    flow_sampling        = 0.5
    metadata             = "INCLUDE_ALL_METADATA"
  }
}

resource "google_vpc_access_connector" "connector" {
  name          = "compliance-connector"
  region        = var.region
  ip_cidr_range = "10.8.0.0/28" # Small range is sufficient for serverless egress
  network       = google_compute_network.vpc.id
  min_instances = 2
  max_instances = 3

  depends_on = [google_project_service.apis]
}

# google_compute_global_address reserves an internal IP range for Private Service Connect.
resource "google_compute_global_address" "private_ip_address" {
  name          = "private-ip-for-sql"
  purpose       = "VPC_PEERING"
  address_type  = "INTERNAL"
  prefix_length = 16
  network       = google_compute_network.vpc.id
}

# google_service_networking_connection establishes peering between the VPC and Google services.
resource "google_service_networking_connection" "private_vpc_connection" {
  network                 = google_compute_network.vpc.id
  service                 = "servicenetworking.googleapis.com"
  reserved_peering_ranges = [google_compute_global_address.private_ip_address.name]
}
