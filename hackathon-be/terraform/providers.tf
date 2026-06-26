provider "azurerm" {
  features {}

  # The azurerm v4 provider requires a subscription id. Set it here, via the
  # `subscription_id` variable, or export ARM_SUBSCRIPTION_ID in your shell.
  subscription_id = var.subscription_id
}
