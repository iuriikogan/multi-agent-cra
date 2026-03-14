# Package variables defines input parameters for the compliance system infrastructure.

variable "project_id" {
  description = "The target GCP project ID."
  type        = string
}

variable "region" {
  description = "The GCP region for deployment."
  type        = string
  default     = "europe-west1"
}

variable "db_password" {
  description = "Password for the Cloud SQL database instance."
  type        = string
  sensitive   = true
}

variable "authorized_users" {
  description = "List of identities permitted to access the dashboard."
  type        = list(string)
  default     = []
}

variable "server_image" {
  description = "Container image URL for the server component."
  type        = string
}

variable "worker_image" {
  description = "Container image URL for the worker component."
  type        = string
}

variable "repo_name" {
  description = "Name of the Artifact Registry repository."
  type        = string
  default     = "multi-agent-cra"
}

variable "cloud_run_server_name" {
  description = "Service name for the Cloud Run frontend."
  type        = string
  default     = "cra-server"
}

variable "cloud_run_worker_name" {
  description = "Service name for the Cloud Run backend worker."
  type        = string
  default     = "cra-worker"
}
