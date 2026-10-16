---
page_title: "pagerly_user Resource"
---

# pagerly_user

A person in the on-call directory: who can be paged, and the number Pagerly dials.

```hcl
resource "pagerly_user" "alice" {
  email        = "alice@example.com"
  name         = "Alice"
  phone_number = "+447700900123"   # or a bare number with country_code
  role         = "member"
}
```

Schema: `email` (Required, forces replacement); `name`, `phone_number`, `country_code`, `role` (Optional). Creating a user that already exists adopts it; changing `role` needs an admin key.

Import: `terraform import pagerly_user.alice alice@example.com`
