output "resource_group_name" {
  description = "Name of the resource group containing all deployed resources."
  value       = azurerm_resource_group.main.name
}

output "acr_login_server" {
  description = "Login server of the Azure Container Registry."
  value       = azurerm_container_registry.main.login_server
}

output "image_ref" {
  description = "Fully-qualified image reference built and deployed."
  value       = local.image_ref
}

output "app_url" {
  description = "Public HTTPS URL of the NightWatch API."
  value       = "https://${azurerm_container_app.api.ingress[0].fqdn}"
}

output "app_health_url" {
  description = "Health-check endpoint URL."
  value       = "https://${azurerm_container_app.api.ingress[0].fqdn}/healthz"
}

output "postgres_fqdn" {
  description = "Fully-qualified domain name of the PostgreSQL Flexible Server."
  value       = azurerm_postgresql_flexible_server.main.fqdn
}

output "database_name" {
  description = "Name of the application database."
  value       = azurerm_postgresql_flexible_server_database.main.name
}

output "database_url" {
  description = "PostgreSQL connection string used by the application (contains credentials)."
  value       = local.database_url
  sensitive   = true
}
