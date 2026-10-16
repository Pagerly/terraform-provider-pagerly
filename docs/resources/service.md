---
page_title: "pagerly_service Resource"
---

# pagerly_service

A service (component) incidents are raised against.

```hcl
resource "pagerly_service" "checkout" {
  name          = "Checkout"
  team_id       = pagerly_team.payments.id
  escalation_id = pagerly_escalation_policy.payments_p1.id
  status        = "active"
}
```

Schema: `name` (Required); `team_id`, `escalation_id`, `status` (Optional); `id` (Read-only).

Import: `terraform import pagerly_service.checkout <service id>`
