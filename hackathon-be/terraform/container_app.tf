############################################
# Observability + Container Apps environment
############################################

resource "azurerm_log_analytics_workspace" "main" {
  name                = "${local.base_name}-law"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  sku                 = "PerGB2018"
  retention_in_days   = 30
  tags                = var.tags
}

resource "azurerm_container_app_environment" "main" {
  name                       = "${local.base_name}-cae"
  resource_group_name        = azurerm_resource_group.main.name
  location                   = azurerm_resource_group.main.location
  log_analytics_workspace_id = azurerm_log_analytics_workspace.main.id
  tags                       = var.tags
}

############################################
# Container App — the NightWatch API
############################################

resource "azurerm_container_app" "api" {
  name                         = "${local.base_name}-api"
  resource_group_name          = azurerm_resource_group.main.name
  container_app_environment_id = azurerm_container_app_environment.main.id
  revision_mode                = "Single"
  tags                         = var.tags

  identity {
    type         = "UserAssigned"
    identity_ids = [azurerm_user_assigned_identity.app.id]
  }

  # Pull from ACR using the user-assigned managed identity (no admin creds).
  registry {
    server   = azurerm_container_registry.main.login_server
    identity = azurerm_user_assigned_identity.app.id
  }

  # Connection string is injected as a secret, then referenced by the env var.
  secret {
    name  = "database-url"
    value = local.database_url
  }

  ingress {
    external_enabled = true
    target_port      = 8080
    transport        = "auto"

    traffic_weight {
      latest_revision = true
      percentage      = 100
    }
  }

  template {
    min_replicas = var.min_replicas
    max_replicas = var.max_replicas

    container {
      name   = "api"
      image  = local.image_ref
      cpu    = var.container_cpu
      memory = var.container_memory

      env {
        name        = "DATABASE_URL"
        secret_name = "database-url"
      }
      env {
        name  = "HTTP_ADDR"
        value = ":8080"
      }
      env {
        name  = "MONITOR_ENABLED"
        value = tostring(var.monitor_enabled)
      }
      env {
        name  = "MONITOR_TICK"
        value = var.monitor_tick
      }
      env {
        name  = "DEFAULT_LOW_BATTERY_THRESHOLD"
        value = tostring(var.default_low_battery_threshold)
      }
      env {
        name  = "LOG_LEVEL"
        value = var.log_level
      }

      liveness_probe {
        transport               = "HTTP"
        port                    = 8080
        path                    = "/healthz"
        initial_delay           = 15
        interval_seconds        = 30
        timeout                 = 5
        failure_count_threshold = 3
      }

      readiness_probe {
        transport               = "HTTP"
        port                    = 8080
        path                    = "/healthz"
        interval_seconds        = 15
        timeout                 = 5
        failure_count_threshold = 3
        success_count_threshold = 1
      }
    }
  }

  depends_on = [
    null_resource.image,
    azurerm_role_assignment.acr_pull,
    azurerm_postgresql_flexible_server_database.main,
    azurerm_postgresql_flexible_server_firewall_rule.azure_services,
  ]
}
