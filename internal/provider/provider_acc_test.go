package provider

import (
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func testFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"pagerly": providerserver.NewProtocol6WithError(New("test")()),
	}
}

const fullConfig = `
resource "pagerly_user" "bob" {
  email = "Bob@Co.com"
  name  = "Bob"
}

resource "pagerly_team" "payments" {
  name     = "Payments"
  timezone = "Europe/London"
  rotation = {
    users     = ["Alice@Co.com", pagerly_user.bob.email]
    frequency = "weekly"
    day       = "monday"
    time      = "09:00"
  }
  reminder = {
    type = "Every Rotation"
  }
}

resource "pagerly_escalation_policy" "p1" {
  name = "Payments P1"
  layer = [
    { team_id = pagerly_team.payments.id },
    { people = [pagerly_user.bob.email], wait_minutes = 10 },
  ]
  repeat = 1
}

resource "pagerly_alert_integration" "datadog" {
  name          = "Datadog prod"
  source        = "datadog"
  escalation_id = pagerly_escalation_policy.p1.id
  rules         = jsonencode([{ name = "ignore info", conditions = [{ field = "severity", op = "is", value = "info" }], action = "ignore" }])
}

resource "pagerly_service" "checkout" {
  name          = "Checkout"
  team_id       = pagerly_team.payments.id
  escalation_id = pagerly_escalation_policy.p1.id
}

resource "pagerly_oncall_group" "backend" {
  name            = "Backend"
  team_ids        = [pagerly_team.payments.id]
  permanent_users = ["Alice@Co.com"]
}

resource "pagerly_team" "infra" {
  name     = "Infra"
  timezone = "UTC"
  sync = {
    source      = "PagerDuty"
    schedule_id = "PABC123"
  }
}

data "pagerly_team" "lookup" {
  name       = "payments"
  depends_on = [pagerly_team.payments]
}
`

const updatedConfig = `
resource "pagerly_user" "bob" {
  email        = "Bob@Co.com"
  name         = "Robert"
  phone_number = "+447700900000"
}

resource "pagerly_team" "payments" {
  name         = "Payments EU"
  timezone     = "Europe/London"
  slack_channel = "C123"
  rotation = {
    users     = [pagerly_user.bob.email, "Alice@Co.com"]
    frequency = "weekly"
    day       = "tuesday"
    time      = "10:00"
  }
}

resource "pagerly_escalation_policy" "p1" {
  name = "Payments P1"
  layer = [
    { team_id = pagerly_team.payments.id, wait_minutes = 15 },
  ]
}

resource "pagerly_alert_integration" "datadog" {
  name    = "Datadog prod"
  source  = "datadog"
  team_id = pagerly_team.payments.id
  alert_only = true
}

resource "pagerly_service" "checkout" {
  name    = "Checkout"
  team_id = pagerly_team.payments.id
  status  = "active"
}

resource "pagerly_oncall_group" "backend" {
  name     = "Backend Core"
  team_ids = [pagerly_team.payments.id, pagerly_team.infra.id]
}

resource "pagerly_team" "infra" {
  name     = "Infra"
  timezone = "UTC"
  sync = {
    source      = "pagerduty"
    schedule_id = "PXYZ789"
  }
}
`

func TestAccProvider_fullLifecycle(t *testing.T) {
	api := newFakeAPI()
	defer api.server.Close()
	t.Setenv("PAGERLY_API_KEY", "test-key")
	t.Setenv("PAGERLY_BASE_URL", api.server.URL)
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 (and TF_ACC_TERRAFORM_PATH to a terraform or tofu binary) to run")
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testFactories(),
		Steps: []resource.TestStep{
			{
				Config: fullConfig,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("pagerly_team.payments", tfjsonpath.New("rotation").AtMapKey("frequency"), knownvalue.StringExact("weekly")),
					statecheck.ExpectKnownValue("pagerly_team.payments", tfjsonpath.New("rotation").AtMapKey("users").AtSliceIndex(0), knownvalue.StringExact("Alice@Co.com")),
					statecheck.ExpectKnownValue("pagerly_team.payments", tfjsonpath.New("current_oncall"), knownvalue.StringExact("alice@co.com")),
					statecheck.ExpectKnownValue("pagerly_escalation_policy.p1", tfjsonpath.New("layer").AtSliceIndex(0).AtMapKey("wait_minutes"), knownvalue.Int64Exact(5)),
					statecheck.ExpectKnownValue("pagerly_escalation_policy.p1", tfjsonpath.New("layer").AtSliceIndex(1).AtMapKey("people").AtSliceIndex(0), knownvalue.StringExact("Bob@Co.com")),
					statecheck.ExpectKnownValue("pagerly_alert_integration.datadog", tfjsonpath.New("webhook_url"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("pagerly_user.bob", tfjsonpath.New("role"), knownvalue.StringExact("member")),
					statecheck.ExpectKnownValue("data.pagerly_team.lookup", tfjsonpath.New("name"), knownvalue.StringExact("Payments")),
					statecheck.ExpectKnownValue("pagerly_team.infra", tfjsonpath.New("sync").AtMapKey("source"), knownvalue.StringExact("PagerDuty")),
					statecheck.ExpectKnownValue("pagerly_team.infra", tfjsonpath.New("rotation"), knownvalue.Null()),
				},
			},
			// A second plan of the same configuration must be empty: that is drift detection.
			{Config: fullConfig, PlanOnly: true, ExpectNonEmptyPlan: false},
			{
				ResourceName:      "pagerly_team.payments",
				ImportState:       true,
				ImportStateVerify: true,
				// The import cannot know the case the configuration used for these.
				ImportStateVerifyIgnore: []string{"rotation.frequency", "rotation.day", "rotation.users"},
			},
			{
				ResourceName:            "pagerly_escalation_policy.p1",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"layer.1.people"},
			},
			{ResourceName: "pagerly_alert_integration.datadog", ImportState: true, ImportStateVerify: true},
			{ResourceName: "pagerly_service.checkout", ImportState: true, ImportStateVerify: true},
			{ResourceName: "pagerly_oncall_group.backend", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"permanent_users"}},
			{ResourceName: "pagerly_user.bob", ImportState: true, ImportStateVerify: true, ImportStateId: "bob@co.com", ImportStateVerifyIgnore: []string{"email", "id"}},
			{
				Config: updatedConfig,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("pagerly_team.payments", tfjsonpath.New("name"), knownvalue.StringExact("Payments EU")),
					statecheck.ExpectKnownValue("pagerly_team.payments", tfjsonpath.New("reminder"), knownvalue.Null()),
					statecheck.ExpectKnownValue("pagerly_team.payments", tfjsonpath.New("rotation").AtMapKey("users").AtSliceIndex(0), knownvalue.StringExact("Bob@Co.com")),
					statecheck.ExpectKnownValue("pagerly_escalation_policy.p1", tfjsonpath.New("repeat"), knownvalue.Int64Exact(0)),
					statecheck.ExpectKnownValue("pagerly_alert_integration.datadog", tfjsonpath.New("escalation_id"), knownvalue.Null()),
					statecheck.ExpectKnownValue("pagerly_alert_integration.datadog", tfjsonpath.New("rules"), knownvalue.Null()),
					statecheck.ExpectKnownValue("pagerly_oncall_group.backend", tfjsonpath.New("id"), knownvalue.StringExact("Backend Core")),
					statecheck.ExpectKnownValue("pagerly_user.bob", tfjsonpath.New("name"), knownvalue.StringExact("Robert")),
					statecheck.ExpectKnownValue("pagerly_team.infra", tfjsonpath.New("sync").AtMapKey("schedule_id"), knownvalue.StringExact("PXYZ789")),
				},
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("pagerly_team.infra", plancheck.ResourceActionUpdate),
					},
				},
			},
			{Config: updatedConfig, PlanOnly: true, ExpectNonEmptyPlan: false},
			{
				Config: strings.Replace(updatedConfig, `sync = {
    source      = "pagerduty"
    schedule_id = "PXYZ789"
  }`, `rotation = {
    users     = ["Alice@Co.com"]
    frequency = "Daily"
  }`, 1),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("pagerly_team.infra", plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("pagerly_team.infra", tfjsonpath.New("sync"), knownvalue.Null()),
				},
			},
		},
	})
}
