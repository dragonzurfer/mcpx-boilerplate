output "cluster_name" {
  value = google_container_cluster.gke.name
}

output "cluster_location" {
  value = google_container_cluster.gke.location
}

output "artifact_registry_repo" {
  value = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.docker.repository_id}"
}

output "static_ip_name" {
  value = google_compute_global_address.lb_ip.name
}

output "static_ip_address" {
  value = google_compute_global_address.lb_ip.address
}

output "app_fqdn" {
  value = local.fqdn
}

output "dns_nameservers" {
  value       = try(google_dns_managed_zone.zone[0].name_servers, [])
  description = "If manage_dns=true, set these as Nameservers in GoDaddy."
}

