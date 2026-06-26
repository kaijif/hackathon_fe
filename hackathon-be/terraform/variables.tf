############################################
# Azure / provider
############################################

variable "subscription_id" {
  description = "Azure subscription ID to deploy into. If null, ARM_SUBSCRIPTION_ID from the environment is used."
  type        = string
  default     = null
}

variable "location" {
  description = "Azure region for all resources."
  type        = string
  default     = "eastus"
}

variable "name_prefix" {
  description = "Prefix used to name resources. Lowercase letters and digits only (used to build a globally-unique ACR name)."
  type        = string
  default     = "nightwatch"

  validation {
    condition     = can(regex("^[a-z][a-z0-9]{1,18}$", var.name_prefix))
    error_message = "name_prefix must be 2-19 chars, start with a letter, and contain only lowercase letters and digits."
  }
}

variable "tags" {
  description = "Tags applied to every resource."
  type        = map(string)
  default = {
    application = "nightwatch"
    managed_by  = "terraform"
  }
}

############################################
# Container registry / image
############################################

variable "acr_sku" {
  description = "SKU for the Azure Container Registry."
  type        = string
  default     = "Basic"
}

variable "image_name" {
  description = "Repository name for the application image inside the registry."
  type        = string
  default     = "nightwatch-api"
}

variable "image_tag" {
  description = "Image tag to build and deploy. When empty, a tag derived from a hash of the application source is used so that source changes trigger a rebuild and a new revision."
  type        = string
  default     = ""
}

############################################
# PostgreSQL Flexible Server
############################################

variable "postgres_version" {
  description = "PostgreSQL major version."
  type        = string
  default     = "16"
}

variable "postgres_sku_name" {
  description = "Compute SKU for the PostgreSQL Flexible Server (e.g. B_Standard_B1ms, GP_Standard_D2s_v3)."
  type        = string
  default     = "B_Standard_B1ms"
}

variable "postgres_storage_mb" {
  description = "Storage size in MB for the PostgreSQL Flexible Server. Must be a value Azure allows (e.g. 32768, 65536, 131072)."
  type        = number
  default     = 32768
}

variable "db_name" {
  description = "Name of the application database created on the server. Matches the database the app's embedded migrations target."
  type        = string
  default     = "nightwatch"
}

variable "db_admin_username" {
  description = "Administrator login for the PostgreSQL Flexible Server."
  type        = string
  default     = "nightwatch"
}

variable "db_admin_password" {
  description = "Administrator password. Leave empty to auto-generate a strong, URL-safe password. If you set one, keep it URL-safe (it is embedded in DATABASE_URL)."
  type        = string
  default     = ""
  sensitive   = true
}

variable "allow_client_ip" {
  description = "Optional public IP address to allow through the PostgreSQL firewall (e.g. your workstation, for ad-hoc psql access). Leave empty to skip."
  type        = string
  default     = ""
}

############################################
# Container App (runtime)
############################################

variable "container_cpu" {
  description = "vCPU allocated to the API container."
  type        = number
  default     = 0.5
}

variable "container_memory" {
  description = "Memory allocated to the API container (must pair validly with container_cpu, e.g. 0.5 vCPU + 1Gi)."
  type        = string
  default     = "1Gi"
}

variable "min_replicas" {
  description = "Minimum number of container replicas. Keep >= 1 so the background monitor scheduler always runs."
  type        = number
  default     = 1
}

variable "max_replicas" {
  description = "Maximum number of container replicas."
  type        = number
  default     = 3
}

############################################
# Application configuration (env vars)
############################################

variable "monitor_enabled" {
  description = "Enable the background check() scheduler (MONITOR_ENABLED)."
  type        = bool
  default     = true
}

variable "monitor_tick" {
  description = "How often the scheduler scans for due nights (MONITOR_TICK)."
  type        = string
  default     = "30s"
}

variable "default_low_battery_threshold" {
  description = "Default low-battery percent for new nights (DEFAULT_LOW_BATTERY_THRESHOLD)."
  type        = number
  default     = 20
}

variable "log_level" {
  description = "Application log level: debug | info | warn | error (LOG_LEVEL)."
  type        = string
  default     = "info"
}
