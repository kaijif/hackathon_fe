# Deploying NightWatch to Azure with Terraform

This module deploys the NightWatch backend to Azure as a container, backed by a
managed PostgreSQL database.

## What it provisions

| Resource | Purpose |
| --- | --- |
| Resource Group | Holds everything below. |
| Azure Container Registry (ACR) | Stores the application image. The existing root `Dockerfile` is built **in the cloud** with `az acr build` — no local Docker daemon needed. |
| User-assigned managed identity + `AcrPull` role | Lets the Container App pull from ACR without admin credentials. |
| PostgreSQL Flexible Server + database | The app's datastore. The server requires TLS, so the connection string uses `sslmode=require`. |
| Log Analytics workspace | Container Apps logs. |
| Container Apps Environment + Container App | Runs the API container, public HTTPS ingress on port 8080, with `/healthz` liveness/readiness probes. |

The application applies its **database schema automatically** on startup via its
embedded migration runner (`internal/db/migrations/*.sql`), so there is no
separate migration step — Terraform only provisions the empty server/database
and wires `DATABASE_URL`.

## Prerequisites

- [Terraform](https://developer.hashicorp.com/terraform/install) >= 1.5
- [Azure CLI](https://learn.microsoft.com/cli/azure/install-azure-cli), logged in:
  ```bash
  az login
  az account set --subscription <SUBSCRIPTION_ID>
  ```
- Permission to create the resources above and to assign roles (the `AcrPull`
  role assignment requires `Microsoft.Authorization/roleAssignments/write`,
  e.g. Owner or User Access Administrator on the subscription/resource group).

## Usage

```bash
cd terraform

# Tell the provider which subscription to use (either of these works):
export ARM_SUBSCRIPTION_ID=<SUBSCRIPTION_ID>
#   ...or set subscription_id in terraform.tfvars

cp terraform.tfvars.example terraform.tfvars   # optional: customize

terraform init
terraform apply
```

On `apply`, Terraform builds and pushes the image, provisions Postgres and the
Container App, and prints the public URL:

```bash
terraform output app_url
curl "$(terraform output -raw app_health_url)"   # -> {"status":"ok"}
```

## Updating the app

The image tag defaults to a hash of the application source (`Dockerfile`,
`go.mod`/`go.sum`, `cmd/`, `internal/`, `api/`). Changing any of those files
changes the tag, so the next `terraform apply` rebuilds the image and rolls out
a new Container App revision automatically. To pin a specific tag instead, set
`image_tag`.

## Configuration

All inputs have sensible defaults — see `variables.tf` and
`terraform.tfvars.example`. Common ones:

- `location` — Azure region (default `eastus`).
- `name_prefix` — resource name prefix and basis for the globally-unique ACR name.
- `postgres_sku_name` / `postgres_storage_mb` — database size.
- `db_admin_password` — leave empty to auto-generate a strong, URL-safe password.
- `container_cpu` / `container_memory` / `min_replicas` / `max_replicas` — runtime sizing.
- `monitor_enabled`, `monitor_tick`, `log_level`, `default_low_battery_threshold` — app env.

`min_replicas` defaults to 1 so the background `check()` scheduler always runs.

## Notes & production hardening

- **Networking:** the database is reached over public networking, restricted to
  Azure-internal traffic via the `AllowAzureServices` (0.0.0.0) firewall rule.
  For production, integrate the Container Apps Environment with a VNet and use a
  Private Endpoint + private DNS for Postgres instead.
- **Secrets:** `DATABASE_URL` (with credentials) is stored as a Container App
  secret and surfaced via `terraform output -raw database_url` (sensitive).
  Consider Azure Key Vault for stronger secret management.
- **APNs push:** disabled here. To enable, supply the `APNS_*` settings and mount
  the `.p8` auth key as a secret volume on the container.

## Destroy

```bash
terraform destroy
```
