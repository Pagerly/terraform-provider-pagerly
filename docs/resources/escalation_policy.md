---
page_title: "pagerly_escalation_policy Resource"
---

# pagerly_escalation_policy

```hcl
resource "pagerly_escalation_policy" "payments_p1" {
  name = "Payments P1"
  layer = [
    { team_id = pagerly_team.payments.id, wait_minutes = 5 },
    { people = ["cto@example.com"], wait_minutes = 10 },
    { team_id = pagerly_team.platform.id },
  ]
  repeat        = 1
  slack_channel = "C0123456789"
}
```

## Schema

- `name` (Required)
- `layer` (Required, 1–3) — each with `team_id` and/or `people` (emails in the user directory), and `wait_minutes` (Layer 1 defaults to 5).
- `repeat` (Optional, 0–9, default 0) — times to start again from layer 1 when nobody acknowledges.
- `slack_channel` (Optional) — where incidents routed to this policy by an alert integration are posted.

A policy that an alert integration still sends alerts to cannot be deleted; point the
integration elsewhere first.

Import: `terraform import pagerly_escalation_policy.p1 <policy id>`
