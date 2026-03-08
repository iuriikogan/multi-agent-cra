variable "project_id" {
  description = "The Google Cloud Project ID"
  type        = string
}

variable "region" {
  description = "The GCP region to deploy to"
  type        = string
  default     = "us-central1"
}

variable "cluster_name" {
  description = "The name of the GKE cluster"
  type        = string
  default     = "agent-engine-cluster"
}

variable "image_repository" {
  description = "Container image repository (e.g., gcr.io/my-project/agent-cra)"
  type        = string
  default     = "gcr.io/google-containers/echoserver:1.10" # Placeholder default
}

variable "gemini_api_key" {
  description = "The Gemini API Key"
  type        = string
  sensitive   = true
}

variable "master_ipv4_cidr_block" {
  description = "The IP range in CIDR notation to use for the hosted master network"
  type        = string
  default     = "172.16.0.0/28"
}
