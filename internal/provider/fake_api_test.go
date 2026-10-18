package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
)

// fakeAPI is an in-memory stand-in for the Pagerly v1 API, close enough to the real
// one to exercise every resource's create, read, update, delete and drift handling.
type fakeAPI struct {
	mu           sync.Mutex
	teams        map[string]map[string]any
	escalations  map[string]map[string]any
	integrations map[string]map[string]any
	services     map[string]map[string]any
	groups       map[string]map[string]any
	users        map[string]map[string]any
	seq          int
	server       *httptest.Server
}

func newFakeAPI() *fakeAPI {
	f := &fakeAPI{
		teams:        map[string]map[string]any{},
		escalations:  map[string]map[string]any{},
		integrations: map[string]map[string]any{},
		services:     map[string]map[string]any{},
		groups:       map[string]map[string]any{},
		users:        map[string]map[string]any{},
	}
	f.server = httptest.NewServer(http.HandlerFunc(f.handle))
	return f
}

func (f *fakeAPI) id(prefix string) string {
	f.seq++
	return fmt.Sprintf("%s-%d", prefix, f.seq)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg, "status": status})
}

func lowerAll(in []any) []any {
	out := make([]any, 0, len(in))
	for _, v := range in {
		out = append(out, strings.ToLower(fmt.Sprint(v)))
	}
	return out
}

func (f *fakeAPI) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Header.Get("X-APIKEY") != "test-key" {
		fail(w, 401, "Invalid API key")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1")
	var body map[string]any
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	parts := strings.SplitN(strings.TrimPrefix(path, "/"), "/", 2)
	collection := parts[0]
	id := ""
	if len(parts) > 1 {
		id = parts[1]
	}
	switch collection {
	case "me":
		writeJSON(w, 200, map[string]any{"org_id": "T1", "workspace_name": "Fake", "key_name": "k", "org_admin": true})
	case "teams":
		f.teams_(w, r, id, body)
	case "escalation-policies":
		f.crud(w, r, f.escalations, "esc", id, body, f.describeEscalation)
	case "alert-integrations":
		f.integrations_(w, r, id, body)
	case "services":
		f.crud(w, r, f.services, "svc", id, body, func(m map[string]any) map[string]any { return m })
	case "oncall-groups":
		f.groups_(w, r, id, body)
	case "users":
		f.users_(w, r, id, body)
	default:
		fail(w, 404, "no such collection "+collection)
	}
}

// crud is the generic id-keyed collection.
func (f *fakeAPI) crud(w http.ResponseWriter, r *http.Request, store map[string]map[string]any, prefix, id string,
	body map[string]any, describe func(map[string]any) map[string]any) {
	switch {
	case r.Method == "GET" && id == "":
		out := []map[string]any{}
		for _, v := range store {
			out = append(out, describe(v))
		}
		writeJSON(w, 200, out)
	case r.Method == "POST":
		body["id"] = f.id(prefix)
		store[body["id"].(string)] = body
		writeJSON(w, 201, describe(body))
	case r.Method == "GET":
		if v, ok := store[id]; ok {
			writeJSON(w, 200, describe(v))
		} else {
			fail(w, 404, "No "+prefix+" with id "+id)
		}
	case r.Method == "PUT":
		v, ok := store[id]
		if !ok {
			fail(w, 404, "No "+prefix+" with id "+id)
			return
		}
		for k, val := range body {
			v[k] = val
		}
		writeJSON(w, 200, describe(v))
	case r.Method == "DELETE":
		delete(store, id)
		w.WriteHeader(204)
	}
}

func (f *fakeAPI) describeEscalation(m map[string]any) map[string]any {
	out := map[string]any{"id": m["id"], "name": m["name"], "repeat": m["repeat"], "slack_channel": m["slack_channel"]}
	layers := []map[string]any{}
	if raw, ok := m["layers"].([]any); ok {
		for i, l := range raw {
			lm := l.(map[string]any)
			people := []any{}
			if p, ok := lm["people"].([]any); ok {
				people = lowerAll(p)
			}
			wait := lm["wait_minutes"]
			if wait == nil && i == 0 {
				wait = float64(5)
			}
			layers = append(layers, map[string]any{"team_id": lm["team_id"], "people": people, "wait_minutes": wait})
		}
	}
	out["layers"] = layers
	return out
}

func (f *fakeAPI) describeTeam(m map[string]any) map[string]any {
	out := map[string]any{
		"id": m["id"], "name": m["name"], "timezone": m["timezone"],
		"slack_channel": m["slack_channel"], "manager_id": m["manager_id"], "jira_board": m["jira_board"],
		"pagerduty_id": m["pagerduty_id"], "custom_slack_message": m["custom_slack_message"],
		"daily_message_time": m["daily_message_time"],
		"update_slack_topic": m["update_slack_topic"] == true, "show_change_notification": m["show_change_notification"] == true,
		"user_group": m["user_group"], "reminder": m["reminder"], "deleted": false, "current_oncall": "alice@co.com",
	}
	if sync, ok := m["sync"].(map[string]any); ok && m["rotation"] == nil {
		out["sync"] = map[string]any{"source": strings.ToLower(fmt.Sprint(sync["source"])), "schedule_id": sync["schedule_id"]}
	}
	if rot, ok := m["rotation"].(map[string]any); ok {
		users := []any{}
		if u, ok := rot["users"].([]any); ok {
			users = lowerAll(u)
		}
		day := rot["day"]
		if day == nil {
			day = "Monday"
		}
		t := rot["time"]
		if t == nil {
			t = "09:00"
		}
		start := rot["start_date"]
		if start == nil {
			start = "2026-10-15"
		}
		// The engine stores the canonical case of the frequency.
		freq := strings.Title(strings.ToLower(fmt.Sprint(rot["frequency"])))
		if strings.EqualFold(freq, "biweekly") {
			freq = "BiWeekly"
		}
		out["rotation"] = map[string]any{"users": users, "frequency": freq, "day": day, "time": t, "start_date": start}
	}
	return out
}

func (f *fakeAPI) teams_(w http.ResponseWriter, r *http.Request, id string, body map[string]any) {
	if r.Method == "GET" && id == "" && r.URL.Query().Get("name") != "" {
		out := []map[string]any{}
		for _, t := range f.teams {
			if strings.EqualFold(fmt.Sprint(t["name"]), r.URL.Query().Get("name")) {
				out = append(out, f.describeTeam(t))
			}
		}
		writeJSON(w, 200, out)
		return
	}
	if r.Method == "PUT" {
		// Like the real API: a null reminder removes it, a null user_group detaches it.
		if v, ok := f.teams[id]; ok {
			if rem, present := body["reminder"]; present && rem == nil {
				delete(v, "reminder")
				delete(body, "reminder")
			}
			if ug, present := body["user_group"]; present && ug == nil {
				delete(v, "user_group")
				delete(body, "user_group")
			}
		}
	}
	f.crud(w, r, f.teams, "team", id, body, f.describeTeam)
}

func (f *fakeAPI) integrations_(w http.ResponseWriter, r *http.Request, id string, body map[string]any) {
	if r.Method == "POST" {
		body["webhook_url"] = "https://api.pagerly.io/webhook/pagerly/integ/" + fmt.Sprint(f.seq+1)
	}
	// Like the real API, every stored rule gets a server-assigned id.
	if rules, ok := body["rules"].([]any); ok {
		for i, rule := range rules {
			if rm, ok := rule.(map[string]any); ok {
				rm["id"] = fmt.Sprintf("rule-%d", i)
			}
		}
	}
	f.crud(w, r, f.integrations, "integ", id, body, func(m map[string]any) map[string]any {
		rules := m["rules"]
		if rules == nil {
			rules = []any{}
		}
		return map[string]any{"id": m["id"], "name": m["name"], "source": m["source"], "team_id": m["team_id"],
			"escalation_id": m["escalation_id"], "alert_only": m["alert_only"] == true, "rules": rules,
			"webhook_url": m["webhook_url"], "connected": false}
	})
}

func (f *fakeAPI) groups_(w http.ResponseWriter, r *http.Request, name string, body map[string]any) {
	describe := func(m map[string]any) map[string]any {
		people := []any{}
		if p, ok := m["permanent_users"].([]any); ok {
			people = lowerAll(p)
		}
		return map[string]any{"name": m["name"], "team_ids": m["team_ids"], "team_names": m["team_ids"],
			"user_group": m["user_group"], "permanent_users": people}
	}
	switch r.Method {
	case "POST":
		f.groups[body["name"].(string)] = body
		writeJSON(w, 201, describe(body))
	case "GET":
		if v, ok := f.groups[name]; ok {
			writeJSON(w, 200, describe(v))
		} else {
			fail(w, 404, "No group on-call named "+name)
		}
	case "PUT":
		if _, ok := f.groups[name]; !ok {
			fail(w, 404, "No group on-call named "+name)
			return
		}
		delete(f.groups, name)
		f.groups[body["name"].(string)] = body
		writeJSON(w, 200, describe(body))
	case "DELETE":
		delete(f.groups, name)
		w.WriteHeader(204)
	}
}

func (f *fakeAPI) users_(w http.ResponseWriter, r *http.Request, email string, body map[string]any) {
	describe := func(m map[string]any) map[string]any {
		role := m["role"]
		if role == nil {
			role = "member"
		}
		return map[string]any{"email": strings.ToLower(fmt.Sprint(m["email"])), "name": m["name"],
			"phone_number": m["phone_number"], "country_code": m["country_code"], "role": role}
	}
	key := strings.ToLower(email)
	switch r.Method {
	case "POST":
		key = strings.ToLower(fmt.Sprint(body["email"]))
		if existing, ok := f.users[key]; ok {
			for k, v := range body {
				if v != nil {
					existing[k] = v
				}
			}
			writeJSON(w, 200, describe(existing))
			return
		}
		f.users[key] = body
		writeJSON(w, 201, describe(body))
	case "GET":
		if v, ok := f.users[key]; ok {
			writeJSON(w, 200, describe(v))
		} else {
			fail(w, 404, "No user with email "+email)
		}
	case "PUT":
		v, ok := f.users[key]
		if !ok {
			fail(w, 404, "No user with email "+email)
			return
		}
		for k, val := range body {
			if val != nil {
				v[k] = val
			}
		}
		writeJSON(w, 200, describe(v))
	case "DELETE":
		delete(f.users, key)
		w.WriteHeader(204)
	}
}
