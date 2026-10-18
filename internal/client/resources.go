package client

import (
	"context"
	"encoding/json"
	"net/http"
)

// Rotation is a team's round-robin definition.
type Rotation struct {
	Users     []string `json:"users"`
	Frequency string   `json:"frequency"`
	Day       string   `json:"day"`
	Time      string   `json:"time"`
	StartDate string   `json:"start_date"`
}

// Sync is the external schedule a synced team follows.
type Sync struct {
	Source     string `json:"source"`
	ScheduleID string `json:"schedule_id"`
}

// Reminder is a team's shift reminder.
type Reminder struct {
	Type            string `json:"type"`
	Day             string `json:"day"`
	Time            string `json:"time"`
	Timezone        string `json:"timezone"`
	MessageTemplate string `json:"message_template"`
	Target          string `json:"target"`
	ChannelList     string `json:"channel_list"`
}

// Team is GET /teams/{id}.
type Team struct {
	ID                     string    `json:"id"`
	Name                   string    `json:"name"`
	Timezone               string    `json:"timezone"`
	SlackChannel           string    `json:"slack_channel"`
	ManagerID              string    `json:"manager_id"`
	JiraBoard              string    `json:"jira_board"`
	CustomSlackMessage     string    `json:"custom_slack_message"`
	DailyMessageTime       string    `json:"daily_message_time"`
	UpdateSlackTopic       bool      `json:"update_slack_topic"`
	ShowChangeNotification bool      `json:"show_change_notification"`
	UserGroup              string    `json:"user_group"`
	Rotation               *Rotation `json:"rotation"`
	Sync                   *Sync     `json:"sync"`
	Reminder               *Reminder `json:"reminder"`
	Deleted                bool      `json:"deleted"`
	CurrentOncall          string    `json:"current_oncall"`
}

// EscalationLayer is one layer of an escalation policy.
type EscalationLayer struct {
	TeamID      string   `json:"team_id"`
	People      []string `json:"people"`
	WaitMinutes *int64   `json:"wait_minutes"`
}

// EscalationPolicy is GET /escalation-policies/{id}.
type EscalationPolicy struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Layers       []EscalationLayer `json:"layers"`
	Repeat       int64             `json:"repeat"`
	SlackChannel string            `json:"slack_channel"`
}

// AlertIntegration is GET /alert-integrations/{id}.
type AlertIntegration struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Source       string          `json:"source"`
	TeamID       string          `json:"team_id"`
	EscalationID string          `json:"escalation_id"`
	AlertOnly    bool            `json:"alert_only"`
	Rules        json.RawMessage `json:"rules"`
	WebhookURL   string          `json:"webhook_url"`
	Connected    bool            `json:"connected"`
}

// Service is GET /services/{id}.
type Service struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	TeamID       string `json:"team_id"`
	EscalationID string `json:"escalation_id"`
	Status       string `json:"status"`
}

// OncallGroup is GET /oncall-groups/{name}.
type OncallGroup struct {
	Name           string   `json:"name"`
	TeamIDs        []string `json:"team_ids"`
	TeamNames      []string `json:"team_names"`
	UserGroup      string   `json:"user_group"`
	PermanentUsers []string `json:"permanent_users"`
}

// User is GET /users/{email}.
type User struct {
	Email       string `json:"email"`
	Name        string `json:"name"`
	PhoneNumber string `json:"phone_number"`
	CountryCode string `json:"country_code"`
	Role        string `json:"role"`
}

// Get is GET path into out.
func (c *Client) Get(ctx context.Context, path string, out any) error {
	return c.Do(ctx, http.MethodGet, path, nil, nil, out)
}

// Post is POST path with body into out.
func (c *Client) Post(ctx context.Context, path string, body, out any) error {
	return c.Do(ctx, http.MethodPost, path, nil, body, out)
}

// Put is PUT path with body into out.
func (c *Client) Put(ctx context.Context, path string, body, out any) error {
	return c.Do(ctx, http.MethodPut, path, nil, body, out)
}

// Delete is DELETE path. A 404 is not an error: the thing is gone either way.
func (c *Client) Delete(ctx context.Context, path string) error {
	err := c.Do(ctx, http.MethodDelete, path, nil, nil, nil)
	if IsNotFound(err) {
		return nil
	}
	return err
}
