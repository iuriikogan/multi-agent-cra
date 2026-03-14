variable "project_id" {
  description = "The Google Cloud Project ID"
  type        = string
}

variable "region" {
  description = "The GCP region to deploy to"
  type        = string
  default     = "us-central1"
}

variable "cloud_run_server_name" {
  description = "Name for the Cloud Run server service"
  type        = string
  default     = "cra-server"
}

variable "cloud_run_worker_name" {
  description = "Name for the Cloud Run worker service"
  type        = string
  default     = "cra-worker"
}

variable "gcs_bucket_name" {
  description = "Name for the GCS bucket for code storage"
  type        = string
  default     = "" # If empty, a unique bucket will be created
}

variable "server_image" {
  description = "Container image for the server service"
  type        = string
}

variable "worker_image" {
  description = "Container image for the worker service"
  type        = string
}

variable "gemini_api_key" {
  description = "The Gemini API Key"
  type        = string
  sensitive   = true
}

variable "db_version" {
  description = "The version of the Cloud SQL PostgreSQL instance"
  type        = string
  default     = "POSTGRES_13"
}

variable "db_tier" {
  description = "The tier for the Cloud SQL instance"
  type        = string
  default     = "db-g1-small"
}

variable "db_user" {
  description = "The username for the Cloud SQL database"
  type        = string
  default     = "cra_user"
}

variable "db_password" {
  description = "The password for the Cloud SQL database"
  type        = string
  sensitive   = true
  default     = "change_me_in_production"
}
