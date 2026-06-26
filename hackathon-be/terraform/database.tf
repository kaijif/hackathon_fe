############################################
# PostgreSQL Flexible Server
#
# The application connects via DATABASE_URL and applies its embedded schema
# (internal/db/migrations/*.sql) automatically on startup, so no separate
# migration step is required here — this just provisions the server + database.
############################################

resource "azurerm_postgresql_flexible_server" "main" {
  name                = "${local.base_name}-pg-${random_string.suffix.result}"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location

  version                = var.postgres_version
  sku_name               = var.postgres_sku_name
  storage_mb             = var.postgres_storage_mb
  administrator_login    = var.db_admin_username
  administrator_password = local.db_password

  # Disable HA/backup extras to keep this a cheap hackathon footprint.
  backup_retention_days = 7

  tags = var.tags

  lifecycle {
    # Storage can auto-grow on the server side; ignore that drift.
    ignore_changes = [zone]
  }
}

resource "azurerm_postgresql_flexible_server_database" "main" {
  name      = var.db_name
  server_id = azurerm_postgresql_flexible_server.main.id
  collation = "en_US.utf8"
  charset   = "UTF8"
}

# Allow connections originating from Azure services (the 0.0.0.0 special rule).
# Container Apps' outbound traffic comes from Azure IP space, so this lets the
# API reach the database without putting both behind a VNet. For production,
# prefer VNet integration + a private endpoint instead.
resource "azurerm_postgresql_flexible_server_firewall_rule" "azure_services" {
  name             = "AllowAzureServices"
  server_id        = azurerm_postgresql_flexible_server.main.id
  start_ip_address = "0.0.0.0"
  end_ip_address   = "0.0.0.0"
}

# Optional: allow a single operator IP for ad-hoc psql access.
resource "azurerm_postgresql_flexible_server_firewall_rule" "client" {
  count            = var.allow_client_ip != "" ? 1 : 0
  name             = "AllowClientIP"
  server_id        = azurerm_postgresql_flexible_server.main.id
  start_ip_address = var.allow_client_ip
  end_ip_address   = var.allow_client_ip
}
