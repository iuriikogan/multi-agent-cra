resource "google_project_service" "firestore" {
  project = var.project_id
  service = "firestore.googleapis.com"
  disable_on_destroy = false
}

resource "google_firestore_database" "database" {
  project     = var.project_id
  name        = "(default)"
  location_id = var.region # Ensure this matches valid Firestore locations
  type        = "FIRESTORE_NATIVE"

  depends_on = [google_project_service.firestore]
}
