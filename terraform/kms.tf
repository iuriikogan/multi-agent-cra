# Enable Secret Manager and KMS APIs
resource "google_project_service" "secretmanager" {
  project            = var.project_id
  service            = "secretmanager.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "cloudkms" {
  project            = var.project_id
  service            = "cloudkms.googleapis.com"
  disable_on_destroy = false
}

# KMS Key Ring
resource "google_kms_key_ring" "key_ring" {
  name       = "${var.cluster_name}-key-ring"
  location   = var.region
  project    = var.project_id
  depends_on = [google_project_service.cloudkms]
}

# KMS Crypto Key
resource "google_kms_crypto_key" "secret_key" {
  name            = "gemini-api-key-encryption"
  key_ring        = google_kms_key_ring.key_ring.id
  purpose         = "ENCRYPT_DECRYPT"
  rotation_period = "7776000s" # 90 days
}

# Grant Secret Manager Service Agent access to the KMS key
data "google_project" "project" {
  project_id = var.project_id
}

resource "google_project_service_identity" "secretmanager_agent" {
  provider = google-beta
  project  = data.google_project.project.project_id
  service  = "secretmanager.googleapis.com"
}

resource "google_kms_crypto_key_iam_member" "sm_sa_encrypter_decrypter" {
  crypto_key_id = google_kms_crypto_key.secret_key.id
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member        = "serviceAccount:${google_project_service_identity.secretmanager_agent.email}"
}
