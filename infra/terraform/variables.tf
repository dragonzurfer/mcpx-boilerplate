variable "project_id" {
  type        = string
  description = "GCP project id to deploy into."
}

variable "region" {
  type        = string
  description = "GCP region for GKE cluster."
  default     = "us-central1"
}

variable "cluster_name" {
  type        = string
  description = "GKE cluster name."
  default     = "mcpx-gke"
}

variable "app_name" {
  type        = string
  description = "App name used for resources."
  default     = "app"
}

variable "artifact_repo_name" {
  type        = string
  description = "Artifact Registry repository name."
  default     = "mcpx-apps"
}

variable "domain" {
  type        = string
  description = "Base domain (no trailing dot), e.g. mcpx.in"
  default     = "mcpx.in"
}

variable "subdomain" {
  type        = string
  description = "Subdomain for this app, e.g. app"
  default     = "app"
}

variable "manage_dns" {
  type        = bool
  description = "If true, Terraform creates Cloud DNS zone + A record for the app."
  default     = true
}
