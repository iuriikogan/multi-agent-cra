# cloud_run.tf - Defines Cloud Run services for server and worker

resource "google_cloud_run_v2_service" "server" {
  name     = var.cloud_run_server_name
  location = var.region
  provider = google-beta
  deletion_protection = false

  template {
    scaling {
      min_instance_count = 0
      max_instance_count = 5
    }

    service_account = google_service_account.sa_reporter.email
    vpc_access {
      connector = google_vpc_access_connector.serverless.id
      egress    = "ALL_TRAFFIC"
    }
    
    containers {
      image = var.server_image
      ports {
        container_port = 8080
      }
      env {
        name  = "ROLE"
        value = "server"
      }
      env {
        name  = "PROJECT_ID"
        value = var.project_id
      }
      env {
        name  = "DATABASE_TYPE"
        value = "CLOUD_SQL"
      }
      env {
        name = "DATABASE_URL"
        value = "host=${google_sql_database_instance.main.private_ip_address} port=5432 user=${var.db_user} password=${var.db_password} dbname=${google_sql_database.main.name} sslmode=require"
      }
      env {
        name = "GEMINI_API_KEY"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.gemini_api_key.secret_id
            version = "latest"
          }
        }
      }
    }
  }

  depends_on = [
    google_project_service.apis,
    google_sql_database_instance.main,
    google_secret_manager_secret_iam_member.compute_sa_access,
    google_secret_manager_secret_version.gemini_api_key_version
  ]
}

resource "google_cloud_run_v2_service" "worker" {
  name     = var.cloud_run_worker_name
  location = var.region
  provider = google-beta

  template {
    scaling {
      min_instance_count = 1 // Ensure at least one worker is always ready
      max_instance_count = 10
    }

    service_account = google_service_account.sa_reporter.email
    vpc_access {
      connector = google_vpc_access_connector.serverless.id
      egress    = "ALL_TRAFFIC"
    }

    containers {
      image = var.worker_image
      # No port exposed for the worker
      env {
        name = "GEMINI_API_KEY"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.gemini_api_key.secret_id
            version = "latest"
          }
        }
      }
      env {
        name  = "PROJECT_ID"
        value = var.project_id
      }
      env {
        name  = "DATABASE_TYPE"
        value = "CLOUD_SQL"
      }
      env {
        name = "DATABASE_URL"
        value = "host=${google_sql_database_instance.main.private_ip_address} port=5432 user=${var.db_user} password=${var.db_password} dbname=${google_sql_database.main.name} sslmode=require"
      }
      env {
        name  = "LOG_LEVEL"
        value = "INFO"
      }
      env {
        name  = "GCS_BUCKET_NAME"
        value = "cra-data-${var.project_id}"
      }
      env {
        name  = "PUBSUB_TOPIC_SCAN_REQUESTS"
        value = "scan-requests"
      }
      env {
        name  = "PUBSUB_SUB_SCAN_REQUESTS"
        value = "scan-requests-sub"
      }
    }
  }
  
  depends_on = [
    google_project_service.apis,
    google_sql_database_instance.main,
    google_secret_manager_secret_iam_member.compute_sa_access,
    google_secret_manager_secret_version.gemini_api_key_version
  ]
}