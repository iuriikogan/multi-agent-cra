# ------------------------------------------------------------------------------
# 1. Kubernetes Service Accounts (KSAs)
# ------------------------------------------------------------------------------

resource "kubernetes_service_account_v1" "ksa_cra_server" {
  metadata {
    name      = "ksa-cra-server"
    namespace = "default"
    annotations = {
      # Use the server SA identity (or reusing classifier/general identity for now)
      "iam.gke.io/gcp-service-account" = google_service_account.sa_classifier.email
    }
  }
}

resource "kubernetes_service_account_v1" "ksa_cra_worker" {
  metadata {
    name      = "ksa-cra-worker"
    namespace = "default"
    annotations = {
      # Use the worker SA identity (reusing auditor for now or create new dedicated one)
      "iam.gke.io/gcp-service-account" = google_service_account.sa_auditor.email
    }
  }
}

# ------------------------------------------------------------------------------
# 2. SecretProviderClass (Connects CSI Driver to Secret Manager)
# ------------------------------------------------------------------------------
resource "kubectl_manifest" "secret_provider_class" {
  yaml_body = yamlencode({
    apiVersion = "secrets-store.csi.x-k8s.io/v1"
    kind       = "SecretProviderClass"
    metadata = {
      name      = "gemini-api-key-spc"
      namespace = "default"
    }
    spec = {
      provider = "gcp"
      parameters = {
        secrets = yamlencode([{
          resourceName = google_secret_manager_secret.gemini_api_key.secret_id
          fileName     = "key"
        }])
      }
      secretObjects = [{
        secretName = "gemini-api-key"
        type       = "Opaque"
        data = [{
          objectName = "key"
          key        = "key"
        }]
      }]
    }
  })
}

# ------------------------------------------------------------------------------
# 3. Deployments
# ------------------------------------------------------------------------------

# --- Unified CRA Server (API + Frontend) ---
resource "kubernetes_deployment_v1" "cra_server" {
  metadata { name = "cra-server" }
  spec {
    replicas = 2
    selector { match_labels = { app = "cra-server" } }
    template {
      metadata { labels = { app = "cra-server" } }
      spec {
        service_account_name = kubernetes_service_account_v1.ksa_cra_server.metadata[0].name
        volume {
          name = "secrets-store-inline"
          csi {
            driver            = "secrets-store.csi.k8s.io"
            read_only         = true
            volume_attributes = { secretProviderClass = "gemini-api-key-spc" }
          }
        }
        container {
          image = "${var.region}-docker.pkg.dev/${var.project_id}/multi-agent-cra/server:latest"
          name  = "server"
          
          # No args needed, defaults to serving API + Static
          
          volume_mount {
            name       = "secrets-store-inline"
            mount_path = "/mnt/secrets-store"
            read_only  = true
          }
          env {
            name = "GEMINI_API_KEY"
            value_from {
              secret_key_ref {
                name = "gemini-api-key"
                key  = "key"
              }
            }
          }
          env {
            name  = "PROJECT_ID"
            value = var.project_id
          }
          env {
            name  = "PUBSUB_TOPIC_SCAN_REQUESTS"
            value = "scan-requests"
          }

          port { container_port = 8080 }

          resources {
            limits = {
              cpu    = "1000m"
              memory = "1Gi"
            }
            requests = {
              cpu    = "500m"
              memory = "512Mi"
            }
          }
          security_context {
            run_as_non_root            = true
            allow_privilege_escalation = false
            capabilities {
              drop = ["ALL"]
            }
          }
          liveness_probe {
            http_get {
              path = "/api/healthz"
              port = 8080
            }
            initial_delay_seconds = 5
            period_seconds        = 10
          }
          image_pull_policy = "Always"
        }
      }
    }
  }
}

# --- CRA Worker (Agents) ---
resource "kubernetes_deployment_v1" "cra_worker" {
  metadata { name = "cra-worker" }
  spec {
    replicas = 1
    selector { match_labels = { app = "cra-worker" } }
    template {
      metadata { labels = { app = "cra-worker" } }
      spec {
        service_account_name = kubernetes_service_account_v1.ksa_cra_worker.metadata[0].name
        volume {
          name = "secrets-store-inline"
          csi {
            driver            = "secrets-store.csi.k8s.io"
            read_only         = true
            volume_attributes = { secretProviderClass = "gemini-api-key-spc" }
          }
        }
        container {
          image = "${var.region}-docker.pkg.dev/${var.project_id}/multi-agent-cra/worker:latest"
          name  = "worker"

          volume_mount {
            name       = "secrets-store-inline"
            mount_path = "/mnt/secrets-store"
            read_only  = true
          }
          env {
            name = "GEMINI_API_KEY"
            value_from {
              secret_key_ref {
                name = "gemini-api-key"
                key  = "key"
              }
            }
          }
          env {
            name  = "PROJECT_ID"
            value = var.project_id
          }
          env {
            name  = "PUBSUB_SUB_SCAN_REQUESTS"
            value = "scan-requests-sub"
          }
          env {
            name  = "PORT"
            value = "8080"
          }

          port { container_port = 8080 } # For health check

          resources {
            limits = {
              cpu    = "1000m"
              memory = "2Gi"
            }
            requests = {
              cpu    = "500m"
              memory = "1Gi"
            }
          }
          security_context {
            run_as_non_root            = true
            allow_privilege_escalation = false
            capabilities {
              drop = ["ALL"]
            }
          }
          liveness_probe {
            http_get {
              path = "/healthz"
              port = 8080
            }
            initial_delay_seconds = 5
            period_seconds        = 10
          }
          image_pull_policy = "Always"
        }
      }
    }
  }
}

# ------------------------------------------------------------------------------
# 4. Services
# ------------------------------------------------------------------------------

resource "kubernetes_service_v1" "svc_server" {
  metadata { name = "agent-cra-service" } # Matching the name expected by Gateway
  spec {
    selector = { app = "cra-server" }
    port {
      port        = 80
      target_port = 8080
    }
    type = "ClusterIP"
  }
}
