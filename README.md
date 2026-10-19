# Terraform Provider for Pagerly

Manage Pagerly on-call configuration as code: teams and their rotations, escalation
policies, alert integrations (inbound webhooks), services, group on-calls and the user
directory. Works with Terraform and OpenTofu.

```hcl
terraform {
  required_providers {
    pagerly = {
      source = "pagerly/pagerly"
    }
  }
}

provider "pagerly" {
  # or PAGERLY_API_KEY
  api_key = var.pagerly_api_key
}

resource "pagerly_user" "alice" {
  email        = "alice@example.com"
  name         = "Alice"
  phone_number = "+447700900123"
}

resource "pagerly_team" "payments" {
  name     = "Payments"
  timezone = "Europe/London"

  rotation = {
    users     = [pagerly_user.alice.email, "bob@example.com"]
    frequency = "Weekly"
    day       = "Monday"
    time      = "09:00"
  }

  reminder = {
    type = "Every Rotation"
  }
}

resource "pagerly_escalation_policy" "payments_p1" {
  name = "Payments P1"
  layer = [
    { team_id = pagerly_team.payments.id, wait_minutes = 5 },
    { people = [pagerly_user.alice.email], wait_minutes = 10 },
  ]
  repeat = 1
}

resource "pagerly_alert_integration" "datadog" {
  name          = "Datadog production"
  source        = "datadog"
  escalation_id = pagerly_escalation_policy.payments_p1.id
}

# A team that follows an existing PagerDuty schedule
resource "pagerly_team" "platform" {
  name     = "Platform"
  timezone = "UTC"
  sync = {
    source      = "pagerduty"
    schedule_id = "PABC123"
  }
}

output "datadog_webhook_url" {
  value = pagerly_alert_integration.datadog.webhook_url
}
```

## Authentication

An organisation admin creates an API key in the Pagerly dashboard under **API Keys**.
The key acts as the admin who created it. Pass it as `api_key` or in the
`PAGERLY_API_KEY` environment variable. `base_url` / `PAGERLY_BASE_URL` points the
provider at another host (defaults to `https://api.pagerly.io`).

## Resources

| Resource | Manages |
|---|---|
| `pagerly_team` | A team: settings, shift reminder, Slack user group, and either a round-robin rotation or a sync from PagerDuty / Opsgenie / Datadog / JSM Ops / Google Calendar |
| `pagerly_escalation_policy` | Up to three layers of who is paged and how long to wait |
| `pagerly_alert_integration` | An inbound alert webhook, where its alerts go, and its routing rules |
| `pagerly_service` | A service / component with its owning team and escalation policy |
| `pagerly_oncall_group` | A group on-call that stands for several teams' on-call |
| `pagerly_user` | A person in the on-call directory |

Data sources: `pagerly_team` (look up by name or id), `pagerly_oncall` (who is on call now).

Overrides, shift swaps, incidents and who is on call are runtime state, not
configuration; they are not managed here. Use the Pagerly MCP server or the dashboard
for those.

## Importing existing configuration

Every resource supports `terraform import` by its id (the email for a user, the name
for a group on-call):

```bash
terraform import pagerly_team.payments 3f2c…
terraform import pagerly_user.alice alice@example.com
```

## Developing

```bash
go build -o bin/terraform-provider-pagerly .
```

Point Terraform or OpenTofu at the local build with a `dev_overrides` block in
`~/.terraformrc` (or `TF_CLI_CONFIG_FILE`):

```hcl
provider_installation {
  dev_overrides {
    "pagerly/pagerly" = "/path/to/terraform-provider-pagerly/bin"
  }
  direct {}
}
```

The acceptance test drives every resource through an in-memory fake of the API:

```bash
TF_ACC=1 TF_ACC_TERRAFORM_PATH=$(which tofu) \
  TF_ACC_PROVIDER_HOST=registry.opentofu.org TF_ACC_PROVIDER_NAMESPACE=hashicorp \
  go test ./internal/provider/ -run TestAccProvider_fullLifecycle
```

(With a Terraform binary instead of OpenTofu, drop the two `TF_ACC_PROVIDER_*` variables.)

`docs/` is generated for the registry from `templates/` and the provider schema. Edit the
template or the schema `Description`, then regenerate (needs `terraform` on the PATH, or it
downloads one); CI fails if `docs/` is stale:

```bash
go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@v0.25.0 generate --provider-name pagerly
```

Releases are cut by pushing a `v*` tag; see [PUBLISHING.md](PUBLISHING.md).

The provider talks to Pagerly's `/api/v1` REST API, which lives in the backend
repository under `services/apiv1`.
