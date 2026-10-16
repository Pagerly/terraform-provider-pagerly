terraform {
  required_providers {
    pagerly = {
      source = "pagerly/pagerly"
    }
  }
}

provider "pagerly" {
  # api_key = var.pagerly_api_key   (or PAGERLY_API_KEY)
}

resource "pagerly_user" "tf_tester" {
  email = "tf-tester@pagerly.io"
  name  = "TF Tester"
}

resource "pagerly_team" "tf_test" {
  name     = "TF Provider Test"
  timezone = "Asia/Kolkata"

  rotation = {
    users     = ["falit@pagerly.io"]
    frequency = "Weekly"
    day       = "Monday"
    time      = "09:00"
  }

  reminder = {
    type = "Every Rotation"
  }
}

resource "pagerly_escalation_policy" "tf_test" {
  name = "TF Provider Test Policy"
  layer = [
    { team_id = pagerly_team.tf_test.id, wait_minutes = 5 },
    { people = [pagerly_user.tf_tester.email], wait_minutes = 10 },
  ]
  repeat = 1
}

resource "pagerly_alert_integration" "tf_test" {
  name          = "TF Provider Test Datadog"
  source        = "datadog"
  escalation_id = pagerly_escalation_policy.tf_test.id
  rules = jsonencode([
    { name = "ignore info", conditions = [{ field = "severity", op = "is", value = "info" }], action = "ignore" }
  ])
}

resource "pagerly_service" "tf_test" {
  name          = "TF Provider Test Service"
  team_id       = pagerly_team.tf_test.id
  escalation_id = pagerly_escalation_policy.tf_test.id
  status        = "active"
}

resource "pagerly_oncall_group" "tf_test" {
  name            = "TF Provider Test Group"
  team_ids        = [pagerly_team.tf_test.id]
  permanent_users = ["falit@pagerly.io"]
}

data "pagerly_team" "by_name" {
  name       = pagerly_team.tf_test.name
  depends_on = [pagerly_team.tf_test]
}

output "webhook_url" { value = pagerly_alert_integration.tf_test.webhook_url }
output "team_id"     { value = data.pagerly_team.by_name.id }
