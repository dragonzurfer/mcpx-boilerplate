provider "google" {
  project = var.project_id
  region  = var.region
}

data "google_project" "project" {
  project_id = var.project_id
}

locals {
  fqdn    = "${var.subdomain}.${var.domain}"
  ip_name = "${var.app_name}-ip"
}

resource "google_compute_network" "vpc" {
  depends_on = [google_project_service.services]

  name                    = "${var.app_name}-vpc"
  auto_create_subnetworks = false
}

resource "google_compute_subnetwork" "subnet" {
  depends_on = [google_project_service.services]

  name          = "${var.app_name}-subnet"
  region        = var.region
  network       = google_compute_network.vpc.id
  ip_cidr_range = "10.10.0.0/16"

  private_ip_google_access = true

  secondary_ip_range {
    range_name    = "${var.app_name}-pods"
    ip_cidr_range = "10.20.0.0/16"
  }

  secondary_ip_range {
    range_name    = "${var.app_name}-services"
    ip_cidr_range = "10.30.0.0/20"
  }
}

resource "google_project_service" "services" {
  for_each = toset([
    "container.googleapis.com",
    "compute.googleapis.com",
    "artifactregistry.googleapis.com",
    "cloudbuild.googleapis.com",
    "dns.googleapis.com",
    "cloudresourcemanager.googleapis.com",
    "iam.googleapis.com",
  ])

  project            = var.project_id
  service            = each.value
  disable_on_destroy = false
}

resource "google_artifact_registry_repository" "docker" {
  depends_on = [google_project_service.services]

  location      = var.region
  repository_id = var.artifact_repo_name
  description   = "Docker images for mcpx apps"
  format        = "DOCKER"
}

resource "google_project_iam_member" "cloudbuild_artifact_writer" {
  depends_on = [google_project_service.services]

  project = var.project_id
  role    = "roles/artifactregistry.writer"
  member  = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"
}

resource "google_container_cluster" "gke" {
  depends_on = [google_project_service.services]

  name                = var.cluster_name
  location            = var.region
  enable_autopilot    = true
  deletion_protection = false

  network    = google_compute_network.vpc.id
  subnetwork = google_compute_subnetwork.subnet.id

  ip_allocation_policy {
    cluster_secondary_range_name  = google_compute_subnetwork.subnet.secondary_ip_range[0].range_name
    services_secondary_range_name = google_compute_subnetwork.subnet.secondary_ip_range[1].range_name
  }
}

resource "google_compute_global_address" "lb_ip" {
  depends_on = [google_project_service.services]

  name = local.ip_name
}

resource "google_dns_managed_zone" "zone" {
  count = var.manage_dns ? 1 : 0

  depends_on = [google_project_service.services]

  name        = replace(var.domain, ".", "-")
  dns_name    = "${var.domain}."
  description = "DNS zone for ${var.domain}"
}

resource "google_dns_record_set" "app_a" {
  count = var.manage_dns ? 1 : 0

  name         = "${local.fqdn}."
  managed_zone = google_dns_managed_zone.zone[0].name
  type         = "A"
  ttl          = 300
  rrdatas      = [google_compute_global_address.lb_ip.address]
}
