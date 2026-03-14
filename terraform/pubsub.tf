# Package pubsub configures messaging infrastructure for the multi-agent orchestration.

resource "google_pubsub_topic" "scan_requests" {
  name = "scan-requests" # Inbound scan triggers from the frontend
}

resource "google_pubsub_subscription" "scan_requests_sub" {
  name  = "scan-requests-sub"
  topic = google_pubsub_topic.scan_requests.name

  push_config {
    push_endpoint = "${google_cloud_run_v2_service.worker.uri}/pubsub/scan-requests"
    oidc_token {
      service_account_email = google_service_account.worker_sa.email
    }
  }
}

resource "google_pubsub_topic" "aggregator" {
  name = "aggregator-topic" # Resource discovery tasks
}

resource "google_pubsub_topic" "modeler" {
  name = "modeler-topic" # Compliance modeling tasks
}

resource "google_pubsub_topic" "validator" {
  name = "validator-topic" # Rule validation tasks
}

resource "google_pubsub_topic" "reviewer" {
  name = "reviewer-topic" # Assessment review tasks
}

resource "google_pubsub_topic" "tagger" {
  name = "tagger-topic" # Resource tagging tasks
}

resource "google_pubsub_topic" "reporter" {
  name = "reporter-topic" # Final report generation tasks
}

resource "google_pubsub_topic" "monitoring" {
  name = "monitoring-topic" # Real-time execution events
}
