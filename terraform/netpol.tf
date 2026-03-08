resource "kubernetes_network_policy" "default_deny" {
  metadata {
    name      = "default-deny"
    namespace = "default"
  }
  spec {
    pod_selector {} # Selects all pods in the namespace
    policy_types = ["Ingress", "Egress"]
  }
}

resource "kubernetes_network_policy" "allow_dns" {
  metadata {
    name      = "allow-dns"
    namespace = "default"
  }
  spec {
    pod_selector {}
    egress {
      ports {
        protocol = "UDP"
        port     = 53
      }
      ports {
        protocol = "TCP"
        port     = 53
      }
      to {
        namespace_selector {
          match_labels = {
            "kubernetes.io/metadata.name" = "kube-system"
          }
        }
      }
    }
    policy_types = ["Egress"]
  }
}

resource "kubernetes_network_policy" "allow_https_egress" {
  metadata {
    name      = "allow-https-egress"
    namespace = "default"
  }
  spec {
    pod_selector {}
    egress {
      ports {
        protocol = "TCP"
        port     = 443
      }
      to {
        ip_block {
          cidr   = "0.0.0.0/0"
          except = ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"] # Block private ranges, allow public internet (GCP APIs)
        }
      }
    }
    policy_types = ["Egress"]
  }
}

resource "kubernetes_network_policy" "allow_internal_traffic" {
  metadata {
    name      = "allow-internal-traffic"
    namespace = "default"
  }
  spec {
    pod_selector {}
    ingress {
      from {
        pod_selector {} # Allow traffic from other pods in the same namespace
      }
    }
    egress {
      to {
        pod_selector {}
      }
    }
    policy_types = ["Ingress", "Egress"]
  }
}
