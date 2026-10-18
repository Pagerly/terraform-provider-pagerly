---
page_title: "pagerly_team Resource"
---

# pagerly_team

A team. Its on-call comes either from a round-robin **rotation** that Pagerly runs, or from
an external schedule it **sync**s with: PagerDuty, Opsgenie, Datadog On-Call, JSM Ops or
Google Calendar. Set exactly one of `rotation` or `sync`. People are given as emails.

```hcl
resource "pagerly_team" "payments" {
  name          = "Payments"
  timezone      = "Europe/London"
  slack_channel = "C0123456789"

  rotation = {
    users      = ["alice@example.com", "bob@example.com"]
    frequency  = "Weekly"      # Hourly | Daily | Weekly | BiWeekly | Monthly
    day        = "Monday"      # weekday; or Monday-Friday / Monday-Saturday / All 7 Days for Daily & Hourly; or 1–31 for Monthly
    time       = "09:00"
    start_date = "2026-10-20"
  }

  reminder = {
    type = "Every Rotation"    # Daily | Weekly | Custom | Every Rotation
  }
}
```

### Synced from PagerDuty (or Opsgenie, Datadog, JSM Ops, Google Calendar)

```hcl
resource "pagerly_team" "platform" {
  name          = "Platform"
  timezone      = "UTC"
  slack_channel = "C0123456789"

  sync = {
    source      = "pagerduty"   # pagerduty | opsgenie | datadog | jsmops | gcal
    schedule_id = "PABC123"     # the schedule's id in that tool
  }
}
```

The source must already be connected under **Scheduling Integrations** in Pagerly. The
schedule id is checked against the source when the team is created or the id changes, so a
typo fails the apply instead of creating a team nobody is paged from. Pagerly keeps the
on-call in step with the source on its usual schedule; `current_oncall` shows the last read.

Changing `schedule_id` or `source` updates the team in place. Switching between `rotation`
and `sync` recreates the team.

## Schema

Required: `name`, and exactly one of:
- `rotation` (`users`, `frequency`; optional `day`, `time`, `start_date`)
- `sync` (`source`, `schedule_id`)

Optional: `timezone` (IANA; required with `rotation`), `slack_channel`, `manager_id`, `jira_board`, `custom_slack_message`,
`daily_message_time`, `update_slack_topic`, `show_change_notification`, `user_group` (Slack user
group id, `S…`), `reminder` (`type`; optional `day`, `time`, `timezone`, `message_template`,
`target`, `channel_list`).

Read-only: `id`, `current_oncall`.

Removing `reminder` turns the reminder off; removing `user_group` detaches it. Deleting the
resource soft-deletes the team (recoverable from the dashboard).

Import: `terraform import pagerly_team.payments <team id>`
