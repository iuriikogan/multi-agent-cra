# Enable IAP API
resource "google_project_service" "iap" {
  project            = var.project_id
  service            = "iap.googleapis.com"
  disable_on_destroy = false
}

# IAP Brand and Client (Requires existing Organization configuration usually)
# For this demo, we assume the Brand might exist or we try to create it.
# Note: google_iap_brand creation often fails if one already exists for the project.

resource "google_iap_brand" "project_brand" {
  support_email     = "support@${var.project_id}.iam.gserviceaccount.com"
  application_title = "Agent CRA Internal"
  project           = var.project_id
  depends_on        = [google_project_service.iap]
}

resource "google_iap_client" "gke_client" {
  display_name = "GKE Gateway Client"
  brand        = google_iap_brand.project_brand.name
}

# K8s Secret to store IAP Credentials for the Gateway
resource "kubernetes_secret" "iap_secret" {
  metadata {
    name      = "iap-secret"
    namespace = "default"
  }
  data = {
    client_id     = google_iap_client.gke_client.client_id
    client_secret = google_iap_client.gke_client.secret
  }
}

# BackendPolicy for IAP
resource "kubectl_manifest" "iap_policy" {
  yaml_body = yamlencode({
    apiVersion = "networking.gke.io/v1"
    kind       = "GCPBackendPolicy"
    metadata = {
      name      = "iap-backend-policy"
      namespace = "default"
    }
    spec = {
      default = {
        iap = {
          enabled  = true
          clientID = google_iap_client.gke_client.client_id
          oauthclientSecret = {
            name = kubernetes_secret.iap_secret.metadata[0].name
          }
        }
      }
      targetRef = {
        group = ""
        kind  = "Service"
        name  = "agent-cra-service"
      }
    }
  })
}