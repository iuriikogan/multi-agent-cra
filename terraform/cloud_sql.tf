# Package cloud_sql configures the managed database instance for multi-framework compliance findings.

resource "google_sql_database_instance" "instance" {
  name             = "compliance-mysql-instance"
  region           = var.region
  database_version = "MYSQL_8_0"

  depends_on = [google_service_networking_connection.private_vpc_connection]

  settings {
    tier = "db-f1-micro" # Minimum tier for cost optimization in dev/staging
    ip_configuration {
      ipv4_enabled    = false # Private IP only for enhanced security
      private_network = google_compute_network.vpc.id
      require_ssl     = true # Resolves SNYK-CC-GCP-270
    }
    backup_configuration {
      enabled = true # Resolves SNYK-CC-GCP-283
    }
    database_flags {
      name  = "local_infile"
      value = "off" # Resolves SNYK-CC-GCP-300
    }
    database_flags {
      name  = "cloudsql_iam_authentication"
      value = "on" # Resolves SNYK-CC-GCP-693
    }
    database_flags {
      name  = "skip_show_database"
      value = "on" # Resolves SNYK-CC-GCP-694
    }

  }
  deletion_protection = false # Set to true for production environments
}

resource "google_sql_database" "database" {
  name     = "compliance_db"
  instance = google_sql_database_instance.instance.name
}

resource "google_sql_user" "users" {
  name     = "compliance_user"
  instance = google_sql_database_instance.instance.name
  host     = "%" # Allow connections from any IP within the private VPC range
  password = var.db_password
}

output "db_ip" {
  description = "The private IP address of the Cloud SQL instance."
  value       = google_sql_database_instance.instance.private_ip_address
}