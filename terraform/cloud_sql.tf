# Package cloud_sql configures the managed database instance for findings storage.

resource "google_sql_database_instance" "instance" {
  name             = "cra-db-instance"
  region           = var.region
  database_version = "MYSQL_8_0"

  depends_on = [google_service_networking_connection.private_vpc_connection]

  settings {
    tier = "db-f1-micro" # Minimum tier for cost optimization in dev/staging
    ip_configuration {
      ipv4_enabled    = false # Private IP only for enhanced security
      private_network = google_compute_network.vpc.id
    }
  }
  deletion_protection = false # Set to true for production environments
}

resource "google_sql_database" "database" {
  name     = "cra_db"
  instance = google_sql_database_instance.instance.name
}

resource "google_sql_user" "users" {
  name     = "cra_user"
  instance = google_sql_database_instance.instance.name
  host     = "%" # Allow connections from any IP within the private VPC range
  password = var.db_password
}

output "db_ip" {
  description = "The private IP address of the Cloud SQL instance."
  value       = google_sql_database_instance.instance.private_ip_address
}