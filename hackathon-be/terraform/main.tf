############################################
# Foundation
############################################

resource "random_string" "suffix" {
  length  = 6
  upper   = false
  special = false
  numeric = true
}

resource "random_password" "db" {
  length           = 24
  special          = true
  override_special = "-_" # URL-safe specials only (the password is embedded in DATABASE_URL)
  min_upper        = 2
  min_lower        = 2
  min_numeric      = 2
  min_special      = 2
}

resource "azurerm_resource_group" "main" {
  name     = "${local.base_name}-rg"
  location = var.location
  tags     = var.tags
}

############################################
# Container registry
############################################

resource "azurerm_container_registry" "main" {
  name                = local.acr_name
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  sku                 = var.acr_sku
  admin_enabled       = false # the Container App pulls via a managed identity, not admin creds
  tags                = var.tags
}

# User-assigned identity the Container App uses to pull images from ACR.
# Using a pre-created identity (rather than a system-assigned one) avoids the
# chicken-and-egg problem of needing the AcrPull role to exist before the app
# is created with its image.
resource "azurerm_user_assigned_identity" "app" {
  name                = "${local.base_name}-app-id"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  tags                = var.tags
}

resource "azurerm_role_assignment" "acr_pull" {
  scope                = azurerm_container_registry.main.id
  role_definition_name = "AcrPull"
  principal_id         = azurerm_user_assigned_identity.app.principal_id
}

############################################
# Build & push the application image
#
# Builds the existing Dockerfile in the cloud with `az acr build` (no local
# Docker daemon required). Re-runs whenever the application source hash changes.
############################################

resource "null_resource" "image" {
  triggers = {
    image_ref = local.image_ref
    src_hash  = local.src_hash
  }

  provisioner "local-exec" {
    working_dir = local.src_root
    command     = "az acr build --registry ${azurerm_container_registry.main.name} --image ${var.image_name}:${local.image_tag} --file Dockerfile ."
  }

  depends_on = [azurerm_container_registry.main]
}
