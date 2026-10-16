---
page_title: "pagerly_oncall Data Source"
---

# pagerly_oncall (data source)

Who is on call right now, for one team or all teams.

```hcl
data "pagerly_oncall" "payments" {
  team_id = pagerly_team.payments.id
}

output "oncall_email" { value = data.pagerly_oncall.payments.oncall[0].email }
```

Attributes: `oncall` — list of `{ team_id, team_name, email, name }`.
