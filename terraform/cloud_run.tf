# Package cloud_run defines the serverless compute resources for the regulatory compliance system.

resource "google_cloud_run_v2_service" "server" {
  custom_audiences = ["google-cloud-run"]
  name     = var.cloud_run_server_name
  location = var.region
  ingress  = "INGRESS_TRAFFIC_ALL"

  template {
    max_instance_request_concurrency = 1
    scaling {
      min_instance_count = 0
      max_instance_count = 100
    }
    service_account = google_service_account.server_sa.email
    vpc_access {
      connector = google_vpc_access_connector.connector.id
      egress    = "ALL_TRAFFIC"
    }
    containers {
      image = var.server_image
      env {
        name  = "PROJECT_ID"
        value = var.project_id
      }
      env {
        name  = "ROLE"
        value = "server"
      }
      env {
        name  = "DATABASE_TYPE"
        value = "CLOUD_SQL"
      }
      env {
        name  = "DATABASE_URL"
        value = "compliance_user:${var.db_password}@tcp(${google_sql_database_instance.instance.private_ip_address}:3306)/compliance_db?parseTime=true"
      }
      env {
        name  = "PUBSUB_TOPIC_SCAN_REQUESTS"
        value = google_pubsub_topic.scan_requests.name
      }
      env {
        name  = "PUBSUB_SUB_SCAN_REQUESTS"
        value = google_pubsub_subscription.scan_requests_sub.name
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
}

resource "google_cloud_run_v2_service_iam_member" "server_invoker" {
  for_each = toset(var.authorized_users)
  name     = google_cloud_run_v2_service.server.name
  location = google_cloud_run_v2_service.server.location
  role     = "roles/run.invoker"
  member   = each.value
}

resource "google_cloud_run_v2_service" "worker" {
  custom_audiences = ["google-cloud-run"]
  name     = var.cloud_run_worker_name
  location = var.region
  ingress  = "INGRESS_TRAFFIC_INTERNAL_ONLY"

  template {
    max_instance_request_concurrency = 1
    scaling {
      min_instance_count = 0
      max_instance_count = 100
    }
    service_account = google_service_account.worker_sa.email
    vpc_access {
      connector = google_vpc_access_connector.connector.id
      egress    = "ALL_TRAFFIC"
    }
    containers {
      image = var.worker_image
      env {
        name  = "PROJECT_ID"
        value = var.project_id
      }
      env {
        name  = "ROLE"
        value = "worker"
      }
      env {
        name  = "DATABASE_TYPE"
        value = "CLOUD_SQL"
      }
      env {
        name  = "DATABASE_URL"
        value = "compliance_user:${var.db_password}@tcp(${google_sql_database_instance.instance.private_ip_address}:3306)/compliance_db?parseTime=true"
      }
      env {
        name  = "PUBSUB_TOPIC_SCAN_REQUESTS"
        value = google_pubsub_topic.scan_requests.name
      }
      env {
        name  = "PUBSUB_SUB_SCAN_REQUESTS"
        value = google_pubsub_subscription.scan_requests_sub.name
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
}