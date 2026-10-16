---
page_title: "pagerly_team Resource"
---

# pagerly_team

A team with its round-robin on-call rotation. People are given as emails.

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

## Schema

Required: `name`, `timezone` (IANA), `rotation` (`users`, `frequency`; optional `day`, `time`, `start_date`).

Optional: `slack_channel`, `manager_id`, `jira_board`, `pagerduty_id`, `custom_slack_message`,
`daily_message_time`, `update_slack_topic`, `show_change_notification`, `user_group` (Slack user
group id, `S…`), `reminder` (`type`; optional `day`, `time`, `timezone`, `message_template`,
`target`, `channel_list`).

Read-only: `id`, `current_oncall`.

Removing `reminder` turns the reminder off; removing `user_group` detaches it. A team whose
on-call is synced from PagerDuty, Opsgenie or a calendar has no Pagerly rotation and cannot
be managed here. Deleting the resource soft-deletes the team (recoverable from the dashboard).

Import: `terraform import pagerly_team.payments <team id>`
