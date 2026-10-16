---
page_title: "Pagerly Provider"
description: Manage Pagerly on-call configuration as code.
---

# Pagerly Provider

Manage Pagerly teams and rotations, escalation policies, alert integrations, services,
group on-calls and the user directory from Terraform or OpenTofu.

## Example Usage

```hcl
provider "pagerly" {
  api_key = var.pagerly_api_key # or PAGERLY_API_KEY
}
```

## Schema

- `api_key` (String, Sensitive, Optional) — Organisation API key, created by an org admin under API Keys in the dashboard. Falls back to `PAGERLY_API_KEY`. The key acts as the admin who created it.
- `base_url` (String, Optional) — API host. Falls back to `PAGERLY_BASE_URL`, then `https://api.pagerly.io`.
