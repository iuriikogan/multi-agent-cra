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
resource "google_artifact_registry_repository" "compliance_repo" {
  location      = var.region
  repository_id = "multi-agent-compliance"
  description   = "Docker repository for Multi-Agent Compliance Platform"
  format        = "DOCKER"
  depends_on    = [google_project_service.artifactregistry]
}

# IAM bindings for the repository and logging are now centrally 
# managed in iam.tf using the compliance-build-sa.