# Package secrets manages sensitive information using Google Secret Manager.

# google_secret_manager_secret.gemini_api_key defines the secret container.
# Access is managed via IAM to ensure only authorized services can retrieve the values.
resource "google_secret_manager_secret" "gemini_api_key" {
  secret_id = "GEMINI_API_KEY"
  project   = var.project_id

  replication {
    auto {}
  }
}

# IAM bindings for the Gemini API Key.
# Both server and worker identities require access to retrieve the key at runtime.

resource "google_secret_manager_secret_iam_member" "server_access" {
  secret_id = google_secret_manager_secret.gemini_api_key.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.server_sa.email}"
}

resource "google_secret_manager_secret_iam_member" "worker_access" {
  secret_id = google_secret_manager_secret.gemini_api_key.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.worker_sa.email}"
}

# Legacy agent-specific access (optional, depending on deployment mode).
resource "google_secret_manager_secret_iam_member" "classifier_access" {
  secret_id = google_secret_manager_secret.gemini_api_key.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.sa_classifier.email}"
}
