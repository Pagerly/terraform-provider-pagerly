---
page_title: "pagerly_oncall_group Resource"
---

# pagerly_oncall_group

A group on-call (multi-mention team): one name that stands for the on-call of several
teams, optionally keeping a Slack user group in step and always including some people.

```hcl
resource "pagerly_oncall_group" "backend" {
  name            = "Backend"
  team_ids        = [pagerly_team.payments.id, pagerly_team.platform.id]
  user_group      = "S0123456789"
  permanent_users = ["cto@example.com"]
}
```

Schema: `name` (Required, no `:` `;` `,`), `team_ids` (Required); `user_group`, `permanent_users` (Optional); `id` = name.

Import: `terraform import pagerly_oncall_group.backend Backend`
