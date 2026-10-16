---
page_title: "pagerly_alert_integration Resource"
---

# pagerly_alert_integration

An inbound alert source: a webhook URL a monitoring tool posts to, and where its alerts go.

```hcl
resource "pagerly_alert_integration" "datadog" {
  name          = "Datadog production"
  source        = "datadog"
  escalation_id = pagerly_escalation_policy.payments_p1.id   # or team_id
  alert_only    = false

  rules = jsonencode([
    {
      name       = "Ignore info"
      conditions = [{ field = "severity", op = "is", value = "info" }]
      action     = "ignore"
    },
    {
      name       = "Database alerts to the DB team"
      conditions = [{ field = "title", op = "contains", value = "postgres" }]
      action     = "route"
      teamid     = pagerly_team.database.id
    },
  ])
}

output "datadog_webhook" { value = pagerly_alert_integration.datadog.webhook_url }
```

## Schema

- `name` (Required)
- `source` (Required, forces replacement) — one of `datadog`, `grafana`, `cloudwatch`, `prometheus`, `newrelic`, `dynatrace`, `splunk`, `sentry`, `bugsnag`, `checkly`, `cubeapm`, `elastic`, `honeycomb`, `opsgenie`, `pagerduty`, `pingdom`, `statuscake`, `sumologic`, `webhook`.
- exactly one of `team_id` / `escalation_id` (Required)
- `alert_only` (Optional, default false) — page people without opening an incident.
- `rules` (Optional, JSON) — routing rules checked in order; each has `conditions` (`field` = `severity` | `title` | `description` | a payload path, `op` = `is` | `is_not` | `contains` | `not_contains`, `value`), `action` = `ignore` | `route`, and for `route` a `teamid` or `escalationid` and optional `opens` = `incident` | `alert`.
- `webhook_url` (Read-only) — paste into the monitoring tool.

Import: `terraform import pagerly_alert_integration.datadog <integration id>`
