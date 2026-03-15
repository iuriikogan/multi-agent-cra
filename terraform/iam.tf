# Package iam manages service accounts and permission bindings for the regulatory compliance agents.

# ------------------------------------------------------------------------------
# 1. Server Service Account
# ------------------------------------------------------------------------------
resource "google_service_account" "server_sa" {
  account_id   = "compliance-server-sa"
  display_name = "Identity for the compliance platform server"
}

resource "google_project_iam_member" "server_secret_accessor" {
  project = var.project_id
  role    = "roles/secretmanager.secretAccessor"
  member  = "serviceAccount:${google_service_account.server_sa.email}"
}

resource "google_project_iam_member" "server_sql_client" {
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.server_sa.email}"
}

resource "google_project_iam_member" "server_trace_agent" {
  project = var.project_id
  role    = "roles/cloudtrace.agent"
  member  = "serviceAccount:${google_service_account.server_sa.email}"
}

# ------------------------------------------------------------------------------
# 2. Worker Service Account
# ------------------------------------------------------------------------------
resource "google_service_account" "worker_sa" {
  account_id   = "compliance-worker-sa"
  display_name = "Identity for the compliance platform worker"
}

resource "google_project_iam_member" "worker_secret_accessor" {
  project = var.project_id
  role    = "roles/secretmanager.secretAccessor"
  member  = "serviceAccount:${google_service_account.worker_sa.email}"
}

resource "google_project_iam_member" "worker_sql_client" {
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.worker_sa.email}"
}

resource "google_project_iam_member" "worker_ai_user" {
  project = var.project_id
  role    = "roles/aiplatform.user"
  member  = "serviceAccount:${google_service_account.worker_sa.email}"
}

resource "google_project_iam_member" "worker_asset_viewer" {
  project = var.project_id
  role    = "roles/cloudasset.viewer"
  member  = "serviceAccount:${google_service_account.worker_sa.email}"
}

resource "google_project_iam_member" "worker_trace_agent" {
  project = var.project_id
  role    = "roles/cloudtrace.agent"
  member  = "serviceAccount:${google_service_account.worker_sa.email}"
}

# ------------------------------------------------------------------------------
# 3. Cloud Build Service Account (Insecure Default Replacement)
# ------------------------------------------------------------------------------
resource "google_service_account" "build_sa" {
  account_id   = "compliance-build-sa"
  display_name = "Identity for the compliance platform build pipeline"
}

resource "google_project_iam_member" "build_log_writer" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.build_sa.email}"
}

resource "google_project_iam_member" "build_artifact_writer" {
  project = var.project_id
  role    = "roles/artifactregistry.writer"
  member  = "serviceAccount:${google_service_account.build_sa.email}"
}

resource "google_project_iam_member" "build_run_admin" {
  project = var.project_id
  role    = "roles/run.admin"
  member  = "serviceAccount:${google_service_account.build_sa.email}"
}

resource "google_project_iam_member" "build_iam_user" {
  project = var.project_id
  role    = "roles/iam.serviceAccountUser"
  member  = "serviceAccount:${google_service_account.build_sa.email}"
}

resource "google_project_iam_member" "build_secret_accessor" {
  project = var.project_id
  role    = "roles/secretmanager.secretAccessor"
  member  = "serviceAccount:${google_service_account.build_sa.email}"
}

resource "google_project_iam_member" "build_storage_admin" {
  project = var.project_id
  role    = "roles/storage.admin"
  member  = "serviceAccount:${google_service_account.build_sa.email}"
}

# ------------------------------------------------------------------------------
# 4. Agent-Specific Service Accounts (Legacy/Specific Agents)
# ------------------------------------------------------------------------------
resource "google_service_account" "sa_classifier" {
  account_id   = "sa-classifier"
  display_name = "Agent: Scope Classifier"
}

resource "google_project_iam_member" "classifier_vertex" {
  project = var.project_id
  role    = "roles/aiplatform.user"
  member  = "serviceAccount:${google_service_account.sa_classifier.email}"
}

resource "google_service_account" "sa_auditor" {
  account_id   = "sa-auditor"
  display_name = "Agent: Regulatory Auditor"
}

resource "google_project_iam_member" "auditor_vertex" {
  project = var.project_id
  role    = "roles/aiplatform.user"
  member  = "serviceAccount:${google_service_account.sa_auditor.email}"
}

resource "google_service_account" "sa_vuln" {
  account_id   = "sa-vuln"
  display_name = "Agent: Vulnerability Watchdog"
}

resource "google_project_iam_member" "vuln_vertex" {
  project = var.project_id
  role    = "roles/aiplatform.user"
  member  = "serviceAccount:${google_service_account.sa_vuln.email}"
}

resource "google_service_account" "sa_reporter" {
  account_id   = "sa-reporter"
  display_name = "Agent: Compliance Reporter"
}

resource "google_project_iam_member" "reporter_vertex" {
  project = var.project_id
  role    = "roles/aiplatform.user"
  member  = "serviceAccount:${google_service_account.sa_reporter.email}"
}