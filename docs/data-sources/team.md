---
page_title: "pagerly_team Data Source"
---

# pagerly_team (data source)

```hcl
data "pagerly_team" "payments" {
  name = "Payments"   # or id
}
```

Attributes: `id`, `name`, `timezone`, `slack_channel`, `current_oncall` (email).
