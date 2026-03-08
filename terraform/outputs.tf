output "project_id" {
  description = "Google Cloud Project ID"
  value       = var.project_id
}

output "region" {
  description = "Google Cloud Region"
  value       = var.region
}

output "artifact_registry_repo" {
  description = "Artifact Registry Docker Repository ID"
  value       = google_artifact_registry_repository.cra_repo.repository_id
}

output "pubsub_scan_topic" {
  description = "Pub/Sub Topic for Scan Requests"
  value       = google_pubsub_topic.scan_requests.name
}

output "cluster_endpoint" {
  description = "Cluster Endpoint"
  value       = google_container_cluster.primary.endpoint
}

output "cluster_name" {
  description = "Cluster Name"
  value       = google_container_cluster.primary.name
}

output "gateway_status" {
  description = "Gateway Manifest Status (Check for IP allocation)"
  value       = try(yamldecode(kubectl_manifest.gateway.yaml_incluster).status, "Pending")
}

output "bastion_command" {
  description = "Command to SSH into the Bastion Host"
  value       = "gcloud compute ssh ${google_compute_instance.bastion.name} --zone ${google_compute_instance.bastion.zone} --tunnel-through-iap"
}

output "proxy_command" {
  description = "Command to proxy kubectl through the bastion"
  value       = "gcloud compute ssh ${google_compute_instance.bastion.name} --zone ${google_compute_instance.bastion.zone} --tunnel-through-iap -- -L 8888:${google_container_cluster.primary.endpoint}:443"
}