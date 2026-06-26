locals {
  # Naming. ACR names must be globally unique and alphanumeric only.
  base_name = var.name_prefix
  acr_name  = "${replace(var.name_prefix, "-", "")}${random_string.suffix.result}"

  # --- Application source hashing -------------------------------------------
  # Hash the files that affect the built image so that any source change yields
  # a new image tag (and therefore a new Container App revision). Terraform
  # state and tooling directories are deliberately excluded.
  src_root = "${path.module}/.."
  src_files = sort(tolist(setunion(
    fileset(local.src_root, "Dockerfile"),
    fileset(local.src_root, ".dockerignore"),
    fileset(local.src_root, "go.mod"),
    fileset(local.src_root, "go.sum"),
    fileset(local.src_root, "cmd/**"),
    fileset(local.src_root, "internal/**"),
    fileset(local.src_root, "api/**"),
  )))
  src_hash = sha1(join("", [for f in local.src_files : filesha1("${local.src_root}/${f}")]))

  # Use the caller-provided tag when set, otherwise a short content hash.
  image_tag = var.image_tag != "" ? var.image_tag : substr(local.src_hash, 0, 12)
  image_ref = "${azurerm_container_registry.main.login_server}/${var.image_name}:${local.image_tag}"

  # --- Database -------------------------------------------------------------
  db_password = var.db_admin_password != "" ? var.db_admin_password : random_password.db.result

  # pgx / libpq connection string. Azure PostgreSQL Flexible Server requires TLS,
  # so sslmode=require (unlike the local docker-compose default of sslmode=disable).
  database_url = format(
    "postgres://%s:%s@%s:5432/%s?sslmode=require",
    var.db_admin_username,
    local.db_password,
    azurerm_postgresql_flexible_server.main.fqdn,
    var.db_name,
  )
}
