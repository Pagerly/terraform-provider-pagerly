package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/pagerly/terraform-provider-pagerly/internal/client"
)

var (
	_ resource.Resource                     = &teamResource{}
	_ resource.ResourceWithConfigure        = &teamResource{}
	_ resource.ResourceWithImportState      = &teamResource{}
	_ resource.ResourceWithConfigValidators = &teamResource{}
	_ resource.ResourceWithValidateConfig   = &teamResource{}
)

func newTeamResource() resource.Resource { return &teamResource{} }

type teamResource struct {
	client *client.Client
}

type teamModel struct {
	ID                     types.String `tfsdk:"id"`
	Name                   types.String `tfsdk:"name"`
	Timezone               types.String `tfsdk:"timezone"`
	SlackChannel           types.String `tfsdk:"slack_channel"`
	ManagerID              types.String `tfsdk:"manager_id"`
	JiraBoard              types.String `tfsdk:"jira_board"`
	CustomSlackMessage     types.String `tfsdk:"custom_slack_message"`
	DailyMessageTime       types.String `tfsdk:"daily_message_time"`
	UpdateSlackTopic       types.Bool   `tfsdk:"update_slack_topic"`
	ShowChangeNotification types.Bool   `tfsdk:"show_change_notification"`
	UserGroup              types.String `tfsdk:"user_group"`
	Rotation               types.Object `tfsdk:"rotation"`
	Sync                   types.Object `tfsdk:"sync"`
	Reminder               types.Object `tfsdk:"reminder"`
	CurrentOncall          types.String `tfsdk:"current_oncall"`
}

type rotationModel struct {
	Users     types.List   `tfsdk:"users"`
	Frequency types.String `tfsdk:"frequency"`
	Day       types.String `tfsdk:"day"`
	Time      types.String `tfsdk:"time"`
	StartDate types.String `tfsdk:"start_date"`
}

type syncModel struct {
	Source     types.String `tfsdk:"source"`
	ScheduleID types.String `tfsdk:"schedule_id"`
}

var syncAttrTypes = map[string]attr.Type{
	"source":      types.StringType,
	"schedule_id": types.StringType,
}

type reminderModel struct {
	Type            types.String `tfsdk:"type"`
	Day             types.String `tfsdk:"day"`
	Time            types.String `tfsdk:"time"`
	Timezone        types.String `tfsdk:"timezone"`
	MessageTemplate types.String `tfsdk:"message_template"`
	Target          types.String `tfsdk:"target"`
	ChannelList     types.String `tfsdk:"channel_list"`
}

var rotationAttrTypes = map[string]attr.Type{
	"users":      types.ListType{ElemType: types.StringType},
	"frequency":  types.StringType,
	"day":        types.StringType,
	"time":       types.StringType,
	"start_date": types.StringType,
}

var reminderAttrTypes = map[string]attr.Type{
	"type":             types.StringType,
	"day":              types.StringType,
	"time":             types.StringType,
	"timezone":         types.StringType,
	"message_template": types.StringType,
	"target":           types.StringType,
	"channel_list":     types.StringType,
}

func (r *teamResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team"
}

func optionalComputedString(description string) schema.StringAttribute {
	return schema.StringAttribute{
		Description:   description,
		Optional:      true,
		Computed:      true,
		PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
	}
}

func (r *teamResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A Pagerly team: either a round-robin rotation run by Pagerly, or a team that follows an " +
			"external schedule (PagerDuty, Opsgenie, Datadog, JSM Ops, Google Calendar). People are given as emails. " +
			"Overrides, swaps and who is on call right now are runtime state and are not managed here.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Team id.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Description: "Team name, unique in the workspace.",
				Required:    true,
			},
			"timezone": schema.StringAttribute{
				Description: "IANA timezone the rotation hands over in, e.g. `Europe/London`. Required with `rotation`; " +
					"optional with `sync`, where the source's own schedule decides the times.",
				Optional: true,
			},
			"slack_channel":        optionalComputedString("Slack channel id the team's on-call updates are posted to."),
			"manager_id":           optionalComputedString("Slack user id of the team's manager."),
			"jira_board":           optionalComputedString("Jira board linked to the team."),
			"custom_slack_message": optionalComputedString("Custom on-call change message."),
			"daily_message_time":   optionalComputedString("Time of day the daily on-call message is sent, `HH:mm`."),
			"update_slack_topic": schema.BoolAttribute{
				Description:   "Keep the channel topic in step with who is on call.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"show_change_notification": schema.BoolAttribute{
				Description:   "Post a message in the channel when on-call changes.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"user_group": schema.StringAttribute{
				Description: "Slack user group id (`S…`) whose membership Pagerly keeps in step with who is on call. " +
					"Remove the attribute to detach it.",
				Optional: true,
			},
			"rotation": schema.SingleNestedAttribute{
				Description: "A Pagerly round-robin rotation. Set exactly one of `rotation` or `sync`; " +
					"switching between them recreates the team.",
				Optional:      true,
				PlanModifiers: []planmodifier.Object{switchesKind()},
				Attributes: map[string]schema.Attribute{
					"users": schema.ListAttribute{
						Description: "Emails of the people on the rotation, in on-call order.",
						Required:    true,
						ElementType: types.StringType,
					},
					"frequency": schema.StringAttribute{
						Description: "How often on-call hands over: `Hourly`, `Daily`, `Weekly`, `BiWeekly` or `Monthly`.",
						Required:    true,
						Validators: []validator.String{
							stringvalidator.OneOfCaseInsensitive("Hourly", "Daily", "Weekly", "BiWeekly", "Monthly"),
						},
					},
					"day": optionalComputedString("For Weekly/BiWeekly the handover weekday (`Monday`); for Daily/Hourly the span " +
						"(`Monday-Friday`, `Monday-Saturday`, `All 7 Days`); for Monthly the day of month (`1`–`31`)."),
					"time":       optionalComputedString("Handover time, `HH:mm`, in the team's timezone."),
					"start_date": optionalComputedString("ISO date the rotation starts from. Defaults to the day the team is created."),
				},
			},
			"sync": schema.SingleNestedAttribute{
				Description: "Follow an external schedule instead of a Pagerly rotation. The source must be connected " +
					"under Scheduling Integrations in Pagerly. Set exactly one of `rotation` or `sync`.",
				Optional:      true,
				PlanModifiers: []planmodifier.Object{switchesKind()},
				Attributes: map[string]schema.Attribute{
					"source": schema.StringAttribute{
						Description: "`pagerduty`, `opsgenie`, `datadog`, `jsmops` or `gcal`.",
						Required:    true,
						Validators: []validator.String{
							stringvalidator.OneOfCaseInsensitive("pagerduty", "opsgenie", "datadog", "jsmops", "gcal"),
						},
					},
					"schedule_id": schema.StringAttribute{
						Description: "The schedule's id in the source (e.g. a PagerDuty schedule id `PABC123`, an Opsgenie " +
							"schedule id, a Google Calendar id).",
						Required: true,
					},
				},
			},
			"reminder": schema.SingleNestedAttribute{
				Description: "Shift reminder. Remove the block to turn reminders off.",
				Optional:    true,
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						Description: "`Daily`, `Weekly`, `Custom` or `Every Rotation`. Custom also needs `day` and `time`.",
						Required:    true,
					},
					"day":              optionalComputedString("Custom only: weekday of the reminder."),
					"time":             optionalComputedString("Custom only: time of the reminder, `HH:mm`."),
					"timezone":         optionalComputedString("Timezone of the reminder. Defaults to the team's."),
					"message_template": optionalComputedString("Custom reminder message template."),
					"target":           optionalComputedString("Where the reminder goes: `channel`, `user` or `both`."),
					"channel_list":     optionalComputedString("Comma-separated channel ids to remind instead of the team channel."),
				},
			},
			"current_oncall": schema.StringAttribute{
				Description: "Email of whoever is on call as of the last read. Informational.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *teamResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(path.MatchRoot("rotation"), path.MatchRoot("sync")),
	}
}

// ValidateConfig: a rotation hands over at a wall-clock time, so it needs a timezone.
func (r *teamResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg teamModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !cfg.Rotation.IsNull() && !cfg.Rotation.IsUnknown() && cfg.Timezone.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("timezone"), "Missing timezone",
			"A team with a rotation needs a timezone, e.g. \"Europe/London\".")
	}
}

// switchesKind recreates the team when it moves between a Pagerly rotation and an external
// schedule: the API refuses to convert one into the other in place.
func switchesKind() planmodifier.Object {
	return objectplanmodifier.RequiresReplaceIf(
		func(_ context.Context, req planmodifier.ObjectRequest, resp *objectplanmodifier.RequiresReplaceIfFuncResponse) {
			if req.State.Raw.IsNull() {
				return
			}
			resp.RequiresReplace = req.StateValue.IsNull() != req.PlanValue.IsNull()
		},
		"Switching between rotation and sync recreates the team.",
		"Switching between `rotation` and `sync` recreates the team.",
	)
}

func (r *teamResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFrom(req, &resp.Diagnostics)
}

// body builds the JSON the API's POST and PUT take from a plan.
func (r *teamResource) body(ctx context.Context, plan *teamModel) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	body := map[string]any{
		"name": plan.Name.ValueString(),
	}
	if p := strPtr(plan.Timezone); p != nil {
		body["timezone"] = *p
	}
	for key, v := range map[string]types.String{
		"slack_channel":        plan.SlackChannel,
		"manager_id":           plan.ManagerID,
		"jira_board":           plan.JiraBoard,
		"custom_slack_message": plan.CustomSlackMessage,
		"daily_message_time":   plan.DailyMessageTime,
	} {
		if p := strPtr(v); p != nil {
			body[key] = *p
		}
	}
	if p := boolPtr(plan.UpdateSlackTopic); p != nil {
		body["update_slack_topic"] = *p
	}
	if p := boolPtr(plan.ShowChangeNotification); p != nil {
		body["show_change_notification"] = *p
	}
	// Null means "no user group": the API detaches one that is set.
	body["user_group"] = strPtr(plan.UserGroup)

	if !plan.Sync.IsNull() && !plan.Sync.IsUnknown() {
		var sync syncModel
		diags.Append(plan.Sync.As(ctx, &sync, basetypesObjectAsOptions)...)
		body["sync"] = map[string]any{
			"source":      sync.Source.ValueString(),
			"schedule_id": sync.ScheduleID.ValueString(),
		}
	}
	if !plan.Rotation.IsNull() && !plan.Rotation.IsUnknown() {
		var rotation rotationModel
		diags.Append(plan.Rotation.As(ctx, &rotation, basetypesObjectAsOptions)...)
		if diags.HasError() {
			return nil, diags
		}
		users, d := stringList(ctx, rotation.Users)
		diags.Append(d...)
		rot := map[string]any{
			"users":     users,
			"frequency": rotation.Frequency.ValueString(),
		}
		for key, v := range map[string]types.String{
			"day": rotation.Day, "time": rotation.Time, "start_date": rotation.StartDate,
		} {
			if p := strPtr(v); p != nil {
				rot[key] = *p
			}
		}
		body["rotation"] = rot
	}

	if plan.Reminder.IsNull() || plan.Reminder.IsUnknown() {
		body["reminder"] = nil
	} else {
		var reminder reminderModel
		diags.Append(plan.Reminder.As(ctx, &reminder, basetypesObjectAsOptions)...)
		rem := map[string]any{"type": reminder.Type.ValueString()}
		for key, v := range map[string]types.String{
			"day": reminder.Day, "time": reminder.Time, "timezone": reminder.Timezone,
			"message_template": reminder.MessageTemplate, "target": reminder.Target, "channel_list": reminder.ChannelList,
		} {
			if p := strPtr(v); p != nil {
				rem[key] = *p
			}
		}
		body["reminder"] = rem
	}
	return body, diags
}

// fill writes an API team into the model, keeping prior values that differ only in
// case or that the API does not echo back (the rotation's start date).
func (r *teamResource) fill(ctx context.Context, m *teamModel, t *client.Team) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(t.ID)
	m.Name = types.StringValue(t.Name)
	m.Timezone = keepFold(m.Timezone, t.Timezone)
	m.SlackChannel = strValue(t.SlackChannel)
	m.ManagerID = strValue(t.ManagerID)
	m.JiraBoard = strValue(t.JiraBoard)
	m.CustomSlackMessage = strValue(t.CustomSlackMessage)
	m.DailyMessageTime = strValue(t.DailyMessageTime)
	m.UpdateSlackTopic = types.BoolValue(t.UpdateSlackTopic)
	m.ShowChangeNotification = types.BoolValue(t.ShowChangeNotification)
	m.UserGroup = strValue(t.UserGroup)
	m.CurrentOncall = strValue(t.CurrentOncall)

	var prior rotationModel
	if !m.Rotation.IsNull() && !m.Rotation.IsUnknown() {
		diags.Append(m.Rotation.As(ctx, &prior, basetypesObjectAsOptions)...)
	}
	if t.Rotation == nil {
		m.Rotation = types.ObjectNull(rotationAttrTypes)
	} else {
		startDate := strValue(t.Rotation.StartDate)
		if !prior.StartDate.IsNull() && !prior.StartDate.IsUnknown() {
			// The engine moves the window as the rotation advances; the date the
			// configuration started it from stays what was written.
			startDate = prior.StartDate
		}
		obj, d := types.ObjectValueFrom(ctx, rotationAttrTypes, rotationModel{
			Users:     listFold(prior.Users, emptyListIfNull(t.Rotation.Users)),
			Frequency: keepFold(prior.Frequency, t.Rotation.Frequency),
			Day:       keepFold(prior.Day, t.Rotation.Day),
			Time:      keepFold(prior.Time, t.Rotation.Time),
			StartDate: startDate,
		})
		diags.Append(d...)
		m.Rotation = obj
	}

	if t.Sync == nil {
		m.Sync = types.ObjectNull(syncAttrTypes)
	} else {
		var priorSync syncModel
		if !m.Sync.IsNull() && !m.Sync.IsUnknown() {
			diags.Append(m.Sync.As(ctx, &priorSync, basetypesObjectAsOptions)...)
		}
		obj, d := types.ObjectValueFrom(ctx, syncAttrTypes, syncModel{
			Source:     keepFold(priorSync.Source, t.Sync.Source),
			ScheduleID: strValue(t.Sync.ScheduleID),
		})
		diags.Append(d...)
		m.Sync = obj
	}

	if t.Reminder == nil {
		m.Reminder = types.ObjectNull(reminderAttrTypes)
	} else {
		var priorRem reminderModel
		if !m.Reminder.IsNull() && !m.Reminder.IsUnknown() {
			diags.Append(m.Reminder.As(ctx, &priorRem, basetypesObjectAsOptions)...)
		}
		obj, d := types.ObjectValueFrom(ctx, reminderAttrTypes, reminderModel{
			Type:            keepFold(priorRem.Type, t.Reminder.Type),
			Day:             keepFold(priorRem.Day, t.Reminder.Day),
			Time:            keepFold(priorRem.Time, t.Reminder.Time),
			Timezone:        keepFold(priorRem.Timezone, t.Reminder.Timezone),
			MessageTemplate: strValue(t.Reminder.MessageTemplate),
			Target:          keepFold(priorRem.Target, t.Reminder.Target),
			ChannelList:     strValue(t.Reminder.ChannelList),
		})
		diags.Append(d...)
		m.Reminder = obj
	}
	return diags
}

func (r *teamResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan teamModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, diags := r.body(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var team client.Team
	if err := r.client.Post(ctx, "/teams", body, &team); err != nil {
		resp.Diagnostics.AddError("Could not create team", err.Error())
		return
	}
	resp.Diagnostics.Append(r.fill(ctx, &plan, &team)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *teamResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state teamModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var team client.Team
	err := r.client.Get(ctx, "/teams/"+client.PathEscape(state.ID.ValueString()), &team)
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Could not read team", err.Error())
		return
	}
	if team.Deleted {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(r.fill(ctx, &state, &team)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *teamResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state teamModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, diags := r.body(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var team client.Team
	if err := r.client.Put(ctx, "/teams/"+client.PathEscape(state.ID.ValueString()), body, &team); err != nil {
		resp.Diagnostics.AddError("Could not update team", err.Error())
		return
	}
	plan.ID = state.ID
	resp.Diagnostics.Append(r.fill(ctx, &plan, &team)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *teamResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state teamModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, "/teams/"+client.PathEscape(state.ID.ValueString())); err != nil {
		resp.Diagnostics.AddError("Could not delete team", err.Error())
	}
}

func (r *teamResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
