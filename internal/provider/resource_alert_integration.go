package provider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/pagerly/terraform-provider-pagerly/internal/client"
)

var (
	_ resource.Resource                = &alertIntegrationResource{}
	_ resource.ResourceWithConfigure   = &alertIntegrationResource{}
	_ resource.ResourceWithImportState = &alertIntegrationResource{}
)

func newAlertIntegrationResource() resource.Resource { return &alertIntegrationResource{} }

type alertIntegrationResource struct {
	client *client.Client
}

type alertIntegrationModel struct {
	ID           types.String         `tfsdk:"id"`
	Name         types.String         `tfsdk:"name"`
	Source       types.String         `tfsdk:"source"`
	TeamID       types.String         `tfsdk:"team_id"`
	EscalationID types.String         `tfsdk:"escalation_id"`
	AlertOnly    types.Bool           `tfsdk:"alert_only"`
	Rules        jsontypes.Normalized `tfsdk:"rules"`
	WebhookURL   types.String         `tfsdk:"webhook_url"`
}

var alertSources = []string{"datadog", "grafana", "cloudwatch", "prometheus", "newrelic", "dynatrace", "splunk",
	"sentry", "bugsnag", "checkly", "cubeapm", "elastic", "honeycomb", "opsgenie", "pagerduty", "pingdom",
	"statuscake", "sumologic", "webhook"}

func (r *alertIntegrationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alert_integration"
}

func (r *alertIntegrationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An inbound alert integration: a webhook URL a monitoring tool posts to, and where its alerts go. " +
			"Exactly one of `team_id` or `escalation_id` is set. The webhook URL is computed; paste it into the tool.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Description: "Integration name as shown in the dashboard.",
				Required:    true,
			},
			"source": schema.StringAttribute{
				Description: "The tool that posts alerts; picks the payload mapping. One of: datadog, grafana, cloudwatch, " +
					"prometheus, newrelic, dynatrace, splunk, sentry, bugsnag, checkly, cubeapm, elastic, honeycomb, " +
					"opsgenie, pagerduty, pingdom, statuscake, sumologic, webhook. Changing it recreates the integration.",
				Required:      true,
				Validators:    []validator.String{stringvalidator.OneOfCaseInsensitive(alertSources...)},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"team_id": schema.StringAttribute{
				Description: "Send alerts to this team's on-call.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(path.MatchRoot("team_id"), path.MatchRoot("escalation_id")),
				},
			},
			"escalation_id": schema.StringAttribute{
				Description: "Send alerts to this escalation policy.",
				Optional:    true,
			},
			"alert_only": schema.BoolAttribute{
				Description: "Page people without opening an incident.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"rules": schema.StringAttribute{
				Description: "JSON list of routing rules checked in order against each alert; see the Pagerly docs for the " +
					"rule shape (`conditions`, `action`, `teamid`/`escalationid`, `opens`). Omit for no rules.",
				Optional:   true,
				CustomType: jsontypes.NormalizedType{},
			},
			"webhook_url": schema.StringAttribute{
				Description:   "URL the monitoring tool posts alerts to.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *alertIntegrationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFrom(req, &resp.Diagnostics)
}

func (r *alertIntegrationResource) body(plan *alertIntegrationModel) (map[string]any, error) {
	body := map[string]any{
		"name":       plan.Name.ValueString(),
		"source":     plan.Source.ValueString(),
		"alert_only": plan.AlertOnly.ValueBool(),
	}
	// Both keys are always sent: moving from a policy to a team clears the policy.
	body["team_id"] = strPtr(plan.TeamID)
	body["escalation_id"] = strPtr(plan.EscalationID)
	if plan.Rules.IsNull() || plan.Rules.IsUnknown() {
		body["rules"] = []any{}
	} else {
		var rules any
		if err := json.Unmarshal([]byte(plan.Rules.ValueString()), &rules); err != nil {
			return nil, err
		}
		body["rules"] = rules
	}
	return body, nil
}

func (r *alertIntegrationResource) fill(m *alertIntegrationModel, i *client.AlertIntegration) {
	m.ID = types.StringValue(i.ID)
	m.Name = types.StringValue(i.Name)
	m.Source = keepFold(m.Source, i.Source)
	m.TeamID = strValue(i.TeamID)
	m.EscalationID = strValue(i.EscalationID)
	m.AlertOnly = types.BoolValue(i.AlertOnly)
	m.WebhookURL = strValue(i.WebhookURL)
	rules := stripRuleIDs(i.Rules)
	if rules == "" || rules == "[]" || rules == "null" {
		m.Rules = jsontypes.NewNormalizedNull()
	} else {
		m.Rules = jsontypes.NewNormalizedValue(rules)
	}
}

// stripRuleIDs drops the id the API assigns to each stored rule: it is not part of
// the configuration, and leaving it in would make every read differ from what was written.
func stripRuleIDs(raw json.RawMessage) string {
	var rules []map[string]any
	if err := json.Unmarshal(raw, &rules); err != nil {
		return string(raw)
	}
	for _, rule := range rules {
		delete(rule, "id")
	}
	out, err := json.Marshal(rules)
	if err != nil {
		return string(raw)
	}
	return string(out)
}

func (r *alertIntegrationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan alertIntegrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, err := r.body(&plan)
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("rules"), "rules is not valid JSON", err.Error())
		return
	}
	var out client.AlertIntegration
	if err := r.client.Post(ctx, "/alert-integrations", body, &out); err != nil {
		resp.Diagnostics.AddError("Could not create alert integration", err.Error())
		return
	}
	r.fill(&plan, &out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *alertIntegrationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state alertIntegrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out client.AlertIntegration
	err := r.client.Get(ctx, "/alert-integrations/"+client.PathEscape(state.ID.ValueString()), &out)
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Could not read alert integration", err.Error())
		return
	}
	r.fill(&state, &out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *alertIntegrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state alertIntegrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, err := r.body(&plan)
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("rules"), "rules is not valid JSON", err.Error())
		return
	}
	delete(body, "source")
	var out client.AlertIntegration
	if err := r.client.Put(ctx, "/alert-integrations/"+client.PathEscape(state.ID.ValueString()), body, &out); err != nil {
		resp.Diagnostics.AddError("Could not update alert integration", err.Error())
		return
	}
	plan.ID = state.ID
	r.fill(&plan, &out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *alertIntegrationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state alertIntegrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, "/alert-integrations/"+client.PathEscape(state.ID.ValueString())); err != nil {
		resp.Diagnostics.AddError("Could not delete alert integration", err.Error())
	}
}

func (r *alertIntegrationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
