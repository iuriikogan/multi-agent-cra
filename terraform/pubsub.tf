resource "google_pubsub_topic" "scan_requests" {
  name = "scan-requests"
}

resource "google_pubsub_subscription" "scan_requests_sub" {
  name  = "scan-requests-sub"
  topic = google_pubsub_topic.scan_requests.name

  ack_deadline_seconds = 600

  push_config {
    push_endpoint = "${google_cloud_run_v2_service.worker.uri}/pubsub/scan-requests"
    oidc_token {
      service_account_email = google_service_account.sa_classifier.email
    }
  }

  retry_policy {
    minimum_backoff = "10s"
    maximum_backoff = "600s"
  }
}

resource "google_pubsub_subscription" "assets_found_sub" {
  name  = "assets-found-sub"
  topic = google_pubsub_topic.assets_found.name
  ack_deadline_seconds = 600

  push_config {
    push_endpoint = "${google_cloud_run_v2_service.worker.uri}/pubsub/aggregator"
    oidc_token {
      service_account_email = google_service_account.sa_classifier.email
    }
  }
}

resource "google_pubsub_subscription" "models_generated_sub" {
  name  = "models-generated-sub"
  topic = google_pubsub_topic.models_generated.name
  ack_deadline_seconds = 600

  push_config {
    push_endpoint = "${google_cloud_run_v2_service.worker.uri}/pubsub/modeler"
    oidc_token {
      service_account_email = google_service_account.sa_classifier.email
    }
  }
}

resource "google_pubsub_subscription" "validation_results_sub" {
  name  = "validation-results-sub"
  topic = google_pubsub_topic.validation_results.name
  ack_deadline_seconds = 600

  push_config {
    push_endpoint = "${google_cloud_run_v2_service.worker.uri}/pubsub/validator"
    oidc_token {
      service_account_email = google_service_account.sa_classifier.email
    }
  }
}

resource "google_pubsub_subscription" "final_reports_sub" {
  name  = "final-reports-sub"
  topic = google_pubsub_topic.final_reports.name
  ack_deadline_seconds = 600

  push_config {
    push_endpoint = "${google_cloud_run_v2_service.worker.uri}/pubsub/reviewer"
    oidc_token {
      service_account_email = google_service_account.sa_classifier.email
    }
  }
}
