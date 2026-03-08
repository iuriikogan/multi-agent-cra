# Enable Artifact Registry API
resource "google_project_service" "artifactregistry" {
  project            = var.project_id
  service            = "artifactregistry.googleapis.com"
  disable_on_destroy = false
}

data "google_project" "project_registry" {
  project_id = var.project_id
}

# Create Artifact Registry Repository for Docker images
resource "google_artifact_registry_repository" "cra_repo" {
  location      = var.region
  repository_id = "multi-agent-cra"
  description   = "Docker repository for Multi-Agent CRA"
  format        = "DOCKER"
  depends_on    = [google_project_service.artifactregistry]
}

# Grant Artifact Registry Writer role to the default Compute Engine service account
# This SA is used by Cloud Build by default
resource "google_artifact_registry_repository_iam_member" "cloudbuild_sa_writer" {
  project    = var.project_id
  location   = google_artifact_registry_repository.cra_repo.location
  repository = google_artifact_registry_repository.cra_repo.name
  role       = "roles/artifactregistry.writer"
  member     = "serviceAccount:${data.google_project.project_registry.number}-compute@developer.gserviceaccount.com"
}

# Grant Logs Writer role to the default Compute Engine service account
# This is required for Cloud Build to write logs
resource "google_project_iam_member" "cloudbuild_sa_logging" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${data.google_project.project_registry.number}-compute@developer.gserviceaccount.com"
}