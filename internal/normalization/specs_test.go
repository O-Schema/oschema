package normalization

import (
	"testing"

	"github.com/O-Schema/oschema/internal/adapters"
	specs "github.com/O-Schema/oschema/configs/specs"
)

func loadSpec(t *testing.T, source string) *adapters.AdapterSpec {
	t.Helper()
	reg := adapters.NewRegistry()
	if err := reg.LoadFS(specs.Embedded); err != nil {
		t.Fatalf("LoadFS: %v", err)
	}
	spec, err := reg.Resolve(source, "")
	if err != nil {
		t.Fatalf("Resolve(%q): %v", source, err)
	}
	return spec
}

func TestNormalizeShopify(t *testing.T) {
	spec := loadSpec(t, "shopify")
	raw := map[string]any{
		"id":          float64(98765),
		"created_at":  "2024-07-15T14:30:00Z",
		"total_price": "349.99",
		"customer": map[string]any{
			"email": "jane@example.com",
		},
		"line_items": []any{
			map[string]any{"title": "Gadget", "quantity": float64(1)},
		},
	}

	evt, err := Normalize(spec, "orders/create", raw)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if evt.Type != "order.created" {
		t.Errorf("Type = %q, want %q", evt.Type, "order.created")
	}
	if evt.ExternalID != "98765" {
		t.Errorf("ExternalID = %q, want %q", evt.ExternalID, "98765")
	}
	if evt.Data["total"] != "349.99" {
		t.Errorf("Data[total] = %v, want %q", evt.Data["total"], "349.99")
	}
	if evt.Data["customer_email"] != "jane@example.com" {
		t.Errorf("Data[customer_email] = %v, want %q", evt.Data["customer_email"], "jane@example.com")
	}
}

func TestNormalizeStripe(t *testing.T) {
	spec := loadSpec(t, "stripe")
	raw := map[string]any{
		"id":          "evt_1abc123",
		"type":        "charge.succeeded",
		"created":     "1720000000",
		"api_version": "2024-01-01",
		"livemode":    true,
		"request": map[string]any{
			"idempotency_key": "idem-123",
		},
		"data": map[string]any{
			"object": map[string]any{
				"id":              "ch_xyz789",
				"object":          "charge",
				"amount":          float64(5000),
				"amount_captured": float64(5000),
				"amount_refunded": float64(0),
				"currency":        "usd",
				"status":          "succeeded",
				"customer":        "cus_abc",
				"description":     "Test charge",
				"payment_intent":  "pi_def",
				"payment_method":  "pm_card_visa",
				"receipt_email":   "buyer@example.com",
				"receipt_url":     "https://pay.stripe.com/receipts/...",
				"billing_details": map[string]any{
					"name":  "Jane Doe",
					"email": "jane@example.com",
					"phone": "+1234567890",
					"address": map[string]any{
						"city":        "San Francisco",
						"country":     "US",
						"postal_code": "94102",
					},
				},
				"payment_method_details": map[string]any{
					"card": map[string]any{
						"brand":   "visa",
						"last4":   "4242",
						"exp_month": float64(12),
						"exp_year":  float64(2025),
						"funding": "credit",
						"network": "visa",
					},
				},
				"outcome": map[string]any{
					"risk_level":     "normal",
					"risk_score":     float64(15),
					"seller_message": "Payment complete.",
				},
			},
		},
	}

	evt, err := Normalize(spec, "charge.succeeded", raw)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if evt.Type != "payment.charge_succeeded" {
		t.Errorf("Type = %q, want %q", evt.Type, "payment.charge_succeeded")
	}
	if evt.ExternalID != "evt_1abc123" {
		t.Errorf("ExternalID = %q, want %q", evt.ExternalID, "evt_1abc123")
	}
	// Core fields
	if evt.Data["object_id"] != "ch_xyz789" {
		t.Errorf("Data[object_id] = %v", evt.Data["object_id"])
	}
	if evt.Data["object_type"] != "charge" {
		t.Errorf("Data[object_type] = %v", evt.Data["object_type"])
	}
	if evt.Data["amount"] != float64(5000) {
		t.Errorf("Data[amount] = %v", evt.Data["amount"])
	}
	if evt.Data["amount_captured"] != float64(5000) {
		t.Errorf("Data[amount_captured] = %v", evt.Data["amount_captured"])
	}
	if evt.Data["description"] != "Test charge" {
		t.Errorf("Data[description] = %v", evt.Data["description"])
	}
	// Billing details
	if evt.Data["billing_name"] != "Jane Doe" {
		t.Errorf("Data[billing_name] = %v", evt.Data["billing_name"])
	}
	if evt.Data["billing_email"] != "jane@example.com" {
		t.Errorf("Data[billing_email] = %v", evt.Data["billing_email"])
	}
	if evt.Data["billing_city"] != "San Francisco" {
		t.Errorf("Data[billing_city] = %v", evt.Data["billing_city"])
	}
	if evt.Data["billing_country"] != "US" {
		t.Errorf("Data[billing_country] = %v", evt.Data["billing_country"])
	}
	// Card details
	if evt.Data["card_brand"] != "visa" {
		t.Errorf("Data[card_brand] = %v", evt.Data["card_brand"])
	}
	if evt.Data["card_last4"] != "4242" {
		t.Errorf("Data[card_last4] = %v", evt.Data["card_last4"])
	}
	if evt.Data["card_funding"] != "credit" {
		t.Errorf("Data[card_funding] = %v", evt.Data["card_funding"])
	}
	// Risk/outcome
	if evt.Data["risk_level"] != "normal" {
		t.Errorf("Data[risk_level] = %v", evt.Data["risk_level"])
	}
	if evt.Data["seller_message"] != "Payment complete." {
		t.Errorf("Data[seller_message] = %v", evt.Data["seller_message"])
	}
	// Envelope fields
	if evt.Data["livemode"] != true {
		t.Errorf("Data[livemode] = %v", evt.Data["livemode"])
	}
	if evt.Data["api_version"] != "2024-01-01" {
		t.Errorf("Data[api_version] = %v", evt.Data["api_version"])
	}
	if evt.Data["idempotency_key"] != "idem-123" {
		t.Errorf("Data[idempotency_key] = %v", evt.Data["idempotency_key"])
	}
}

func TestNormalizeStripe2025(t *testing.T) {
	reg := adapters.NewRegistry()
	if err := reg.LoadFS(specs.Embedded); err != nil {
		t.Fatalf("LoadFS: %v", err)
	}
	spec, err := reg.Resolve("stripe", "2025-04")
	if err != nil {
		t.Fatalf("Resolve stripe 2025-04: %v", err)
	}

	// 2025 spec should have new event types
	if _, ok := spec.TypeMapping["billing.alert.triggered"]; !ok {
		t.Error("2025 spec missing billing.alert.triggered type mapping")
	}
	if _, ok := spec.TypeMapping["entitlements.active_entitlement_summary.updated"]; !ok {
		t.Error("2025 spec missing entitlements type mapping")
	}

	// Should still handle regular events
	raw := map[string]any{
		"id":   "evt_2025",
		"type": "charge.succeeded",
		"data": map[string]any{
			"object": map[string]any{
				"id":     "ch_2025",
				"amount": float64(9900),
			},
		},
	}
	evt, err := Normalize(spec, "charge.succeeded", raw)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if evt.Type != "payment.charge_succeeded" {
		t.Errorf("Type = %q", evt.Type)
	}
}

func TestNormalizeGitHub2025(t *testing.T) {
	reg := adapters.NewRegistry()
	if err := reg.LoadFS(specs.Embedded); err != nil {
		t.Fatalf("LoadFS: %v", err)
	}
	spec, err := reg.Resolve("github", "2025-01")
	if err != nil {
		t.Fatalf("Resolve github 2025-01: %v", err)
	}

	if _, ok := spec.TypeMapping["sub_issues"]; !ok {
		t.Error("2025 spec missing sub_issues type mapping")
	}
}

func TestNormalizeStripeSubscription(t *testing.T) {
	spec := loadSpec(t, "stripe")
	raw := map[string]any{
		"id":   "evt_sub_001",
		"type": "customer.subscription.created",
		"data": map[string]any{
			"object": map[string]any{
				"id":       "sub_abc",
				"customer": "cus_xyz",
				"status":   "active",
			},
		},
	}

	evt, err := Normalize(spec, "customer.subscription.created", raw)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if evt.Type != "subscription.created" {
		t.Errorf("Type = %q, want %q", evt.Type, "subscription.created")
	}
	if evt.Data["status"] != "active" {
		t.Errorf("Data[status] = %v, want %q", evt.Data["status"], "active")
	}
}

func TestNormalizeGitHubPush(t *testing.T) {
	spec := loadSpec(t, "github")
	raw := map[string]any{
		"ref":     "refs/heads/main",
		"before":  "aaa111",
		"after":   "bbb222",
		"created": false,
		"deleted": false,
		"forced":  false,
		"compare": "https://github.com/octocat/hello-world/compare/aaa111...bbb222",
		"pusher":  map[string]any{"name": "octocat", "email": "octo@github.com"},
		"sender":  map[string]any{"login": "octocat", "id": float64(1), "type": "User"},
		"repository": map[string]any{
			"full_name":      "octocat/hello-world",
			"private":        false,
			"default_branch": "main",
			"html_url":       "https://github.com/octocat/hello-world",
		},
		"head_commit": map[string]any{
			"id":        "bbb222",
			"message":   "Update README.md",
			"timestamp": "2024-07-15T14:30:00Z",
			"author":    map[string]any{"name": "Octocat", "email": "octo@github.com"},
		},
	}

	evt, err := Normalize(spec, "push", raw)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if evt.Type != "repo.push" {
		t.Errorf("Type = %q, want %q", evt.Type, "repo.push")
	}
	if evt.Data["repository"] != "octocat/hello-world" {
		t.Errorf("Data[repository] = %v", evt.Data["repository"])
	}
	if evt.Data["sender"] != "octocat" {
		t.Errorf("Data[sender] = %v", evt.Data["sender"])
	}
	if evt.Data["sender_type"] != "User" {
		t.Errorf("Data[sender_type] = %v", evt.Data["sender_type"])
	}
	if evt.Data["ref"] != "refs/heads/main" {
		t.Errorf("Data[ref] = %v", evt.Data["ref"])
	}
	if evt.Data["compare"] != "https://github.com/octocat/hello-world/compare/aaa111...bbb222" {
		t.Errorf("Data[compare] = %v", evt.Data["compare"])
	}
	if evt.Data["repo_default_branch"] != "main" {
		t.Errorf("Data[repo_default_branch] = %v", evt.Data["repo_default_branch"])
	}
	if evt.Data["head_commit_message"] != "Update README.md" {
		t.Errorf("Data[head_commit_message] = %v", evt.Data["head_commit_message"])
	}
	if evt.Data["head_commit_timestamp"] != "2024-07-15T14:30:00Z" {
		t.Errorf("Data[head_commit_timestamp] = %v", evt.Data["head_commit_timestamp"])
	}
	if evt.Data["head_commit_author"] != "Octocat" {
		t.Errorf("Data[head_commit_author] = %v", evt.Data["head_commit_author"])
	}
	if evt.Data["pusher_name"] != "octocat" {
		t.Errorf("Data[pusher_name] = %v", evt.Data["pusher_name"])
	}
	if evt.Data["pusher_email"] != "octo@github.com" {
		t.Errorf("Data[pusher_email] = %v", evt.Data["pusher_email"])
	}
}

func TestNormalizeGitHubPullRequest(t *testing.T) {
	spec := loadSpec(t, "github")
	raw := map[string]any{
		"action": "opened",
		"number": float64(42),
		"sender": map[string]any{"login": "contributor"},
		"repository": map[string]any{
			"full_name": "org/repo",
		},
		"pull_request": map[string]any{
			"title":  "Add feature X",
			"state":  "open",
			"merged": false,
			"user":   map[string]any{"login": "contributor"},
			"head":   map[string]any{"ref": "feature-x"},
			"base":   map[string]any{"ref": "main"},
		},
	}

	evt, err := Normalize(spec, "pull_request", raw)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if evt.Type != "repo.pull_request" {
		t.Errorf("Type = %q, want %q", evt.Type, "repo.pull_request")
	}
	if evt.Data["action"] != "opened" {
		t.Errorf("Data[action] = %v, want %q", evt.Data["action"], "opened")
	}
	if evt.Data["pr_title"] != "Add feature X" {
		t.Errorf("Data[pr_title] = %v", evt.Data["pr_title"])
	}
	if evt.Data["pr_head_ref"] != "feature-x" {
		t.Errorf("Data[pr_head_ref] = %v", evt.Data["pr_head_ref"])
	}
}

func TestNormalizeSlackMessage(t *testing.T) {
	spec := loadSpec(t, "slack")
	raw := map[string]any{
		"type":       "event_callback",
		"event_id":   "Ev0123ABCD",
		"event_time": float64(1720000000),
		"team_id":    "T01234",
		"api_app_id": "A01234",
		"event": map[string]any{
			"type":         "message",
			"user":         "U12345",
			"text":         "Hello world",
			"channel":      "C67890",
			"ts":           "1720000000.123456",
			"channel_type": "channel",
		},
	}

	evt, err := Normalize(spec, "message", raw)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if evt.Type != "chat.message" {
		t.Errorf("Type = %q, want %q", evt.Type, "chat.message")
	}
	if evt.ExternalID != "Ev0123ABCD" {
		t.Errorf("ExternalID = %q, want %q", evt.ExternalID, "Ev0123ABCD")
	}
	if evt.Data["user"] != "U12345" {
		t.Errorf("Data[user] = %v, want %q", evt.Data["user"], "U12345")
	}
	if evt.Data["text"] != "Hello world" {
		t.Errorf("Data[text] = %v, want %q", evt.Data["text"], "Hello world")
	}
	if evt.Data["channel"] != "C67890" {
		t.Errorf("Data[channel] = %v", evt.Data["channel"])
	}
}

func TestNormalizeSlackReaction(t *testing.T) {
	spec := loadSpec(t, "slack")
	raw := map[string]any{
		"event_id":   "Ev999",
		"event_time": float64(1720000100),
		"team_id":    "T01234",
		"api_app_id": "A01234",
		"event": map[string]any{
			"type":     "reaction_added",
			"user":     "U12345",
			"reaction": "thumbsup",
			"item": map[string]any{
				"type":    "message",
				"channel": "C67890",
				"ts":      "1720000000.123456",
			},
		},
	}

	evt, err := Normalize(spec, "reaction_added", raw)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if evt.Type != "chat.reaction_added" {
		t.Errorf("Type = %q, want %q", evt.Type, "chat.reaction_added")
	}
	if evt.Data["reaction"] != "thumbsup" {
		t.Errorf("Data[reaction] = %v, want %q", evt.Data["reaction"], "thumbsup")
	}
	if evt.Data["item_type"] != "message" {
		t.Errorf("Data[item_type] = %v", evt.Data["item_type"])
	}
}

func TestNormalizeJiraIssueCreated(t *testing.T) {
	spec := loadSpec(t, "jira")
	raw := map[string]any{
		"webhookEvent": "jira:issue_created",
		"timestamp":    float64(1720000000000),
		"user": map[string]any{
			"displayName": "John Dev",
		},
		"issue": map[string]any{
			"id":  "10001",
			"key": "PROJ-123",
			"fields": map[string]any{
				"summary":  "Fix login bug",
				"status":   map[string]any{"name": "To Do"},
				"priority": map[string]any{"name": "High"},
				"issuetype": map[string]any{"name": "Bug"},
				"project":  map[string]any{"key": "PROJ"},
				"assignee": map[string]any{
					"displayName":  "Jane Dev",
					"emailAddress": "jane@company.com",
				},
				"reporter": map[string]any{
					"displayName": "John Dev",
				},
			},
		},
	}

	evt, err := Normalize(spec, "jira:issue_created", raw)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if evt.Type != "issue.created" {
		t.Errorf("Type = %q, want %q", evt.Type, "issue.created")
	}
	if evt.ExternalID != "PROJ-123" {
		t.Errorf("ExternalID = %q, want %q", evt.ExternalID, "PROJ-123")
	}
	if evt.Data["summary"] != "Fix login bug" {
		t.Errorf("Data[summary] = %v", evt.Data["summary"])
	}
	if evt.Data["status"] != "To Do" {
		t.Errorf("Data[status] = %v", evt.Data["status"])
	}
	if evt.Data["assignee"] != "Jane Dev" {
		t.Errorf("Data[assignee] = %v", evt.Data["assignee"])
	}
	if evt.Data["actor"] != "John Dev" {
		t.Errorf("Data[actor] = %v", evt.Data["actor"])
	}
}

func TestNormalizeLinearIssueCreated(t *testing.T) {
	spec := loadSpec(t, "linear")
	raw := map[string]any{
		"action":    "create",
		"type":      "Issue",
		"createdAt": "2024-07-15T10:00:00Z",
		"data": map[string]any{
			"id":          "uuid-linear-001",
			"identifier":  "ENG-456",
			"title":       "Implement dark mode",
			"description": "Add dark mode support",
			"priority":    float64(2),
			"state":       map[string]any{"name": "In Progress"},
			"assignee":    map[string]any{"name": "Alice", "email": "alice@company.com"},
			"team":        map[string]any{"name": "Engineering", "key": "ENG"},
			"project":     map[string]any{"name": "UI Redesign"},
			"labels":      []any{map[string]any{"name": "feature"}, map[string]any{"name": "frontend"}},
			"createdAt":   "2024-07-15T09:00:00Z",
			"updatedAt":   "2024-07-15T10:00:00Z",
		},
	}

	evt, err := Normalize(spec, "create", raw)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if evt.Type != "issue.created" {
		t.Errorf("Type = %q, want %q", evt.Type, "issue.created")
	}
	if evt.ExternalID != "uuid-linear-001" {
		t.Errorf("ExternalID = %q, want %q", evt.ExternalID, "uuid-linear-001")
	}
	if evt.Data["identifier"] != "ENG-456" {
		t.Errorf("Data[identifier] = %v", evt.Data["identifier"])
	}
	if evt.Data["title"] != "Implement dark mode" {
		t.Errorf("Data[title] = %v", evt.Data["title"])
	}
	if evt.Data["state"] != "In Progress" {
		t.Errorf("Data[state] = %v", evt.Data["state"])
	}
	if evt.Data["assignee"] != "Alice" {
		t.Errorf("Data[assignee] = %v", evt.Data["assignee"])
	}
	if evt.Data["team"] != "Engineering" {
		t.Errorf("Data[team] = %v", evt.Data["team"])
	}
}

func TestNormalizePagerDutyIncident(t *testing.T) {
	spec := loadSpec(t, "pagerduty")
	raw := map[string]any{
		"event": map[string]any{
			"id":          "webhook-evt-001",
			"event_type":  "incident.triggered",
			"occurred_at": "2024-07-15T03:00:00Z",
			"data": map[string]any{
				"id":       "PABC123",
				"type":     "incident",
				"title":    "Database CPU at 99%",
				"status":   "triggered",
				"urgency":  "high",
				"html_url": "https://pagerduty.com/incidents/PABC123",
				"service": map[string]any{
					"id":      "PSVC001",
					"summary": "Production DB",
				},
				"priority": map[string]any{
					"summary": "P1",
				},
				"escalation_policy": map[string]any{
					"summary": "Database Team",
				},
				"created_at": "2024-07-15T03:00:00Z",
			},
		},
	}

	evt, err := Normalize(spec, "incident.triggered", raw)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if evt.Type != "incident.triggered" {
		t.Errorf("Type = %q, want %q", evt.Type, "incident.triggered")
	}
	if evt.ExternalID != "PABC123" {
		t.Errorf("ExternalID = %q, want %q", evt.ExternalID, "PABC123")
	}
	if evt.Data["title"] != "Database CPU at 99%" {
		t.Errorf("Data[title] = %v", evt.Data["title"])
	}
	if evt.Data["urgency"] != "high" {
		t.Errorf("Data[urgency] = %v", evt.Data["urgency"])
	}
	if evt.Data["service_name"] != "Production DB" {
		t.Errorf("Data[service_name] = %v", evt.Data["service_name"])
	}
	if evt.Data["priority"] != "P1" {
		t.Errorf("Data[priority] = %v", evt.Data["priority"])
	}
}

func TestNormalizeSendGrid(t *testing.T) {
	spec := loadSpec(t, "sendgrid")
	raw := map[string]any{
		"email":         "user@example.com",
		"event":         "delivered",
		"sg_event_id":   "sg-evt-001",
		"sg_message_id": "msg-001",
		"timestamp":     float64(1720000000),
		"category":      "marketing",
		"status":        "250 OK",
	}

	evt, err := Normalize(spec, "delivered", raw)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if evt.Type != "email.delivered" {
		t.Errorf("Type = %q, want %q", evt.Type, "email.delivered")
	}
	if evt.ExternalID != "sg-evt-001" {
		t.Errorf("ExternalID = %q, want %q", evt.ExternalID, "sg-evt-001")
	}
	if evt.Data["email"] != "user@example.com" {
		t.Errorf("Data[email] = %v", evt.Data["email"])
	}
	if evt.Data["message_id"] != "msg-001" {
		t.Errorf("Data[message_id] = %v", evt.Data["message_id"])
	}
}

func TestNormalizeSendGridBounce(t *testing.T) {
	spec := loadSpec(t, "sendgrid")
	raw := map[string]any{
		"email":         "bad@example.com",
		"event":         "bounce",
		"sg_event_id":   "sg-evt-002",
		"sg_message_id": "msg-002",
		"timestamp":     float64(1720000100),
		"reason":        "550 User not found",
		"status":        "550",
	}

	evt, err := Normalize(spec, "bounce", raw)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if evt.Type != "email.bounced" {
		t.Errorf("Type = %q, want %q", evt.Type, "email.bounced")
	}
	if evt.Data["reason"] != "550 User not found" {
		t.Errorf("Data[reason] = %v", evt.Data["reason"])
	}
}

func TestNormalizeDiscord(t *testing.T) {
	spec := loadSpec(t, "discord")
	raw := map[string]any{
		"type":           float64(2),
		"id":             "interaction-001",
		"application_id": "app-001",
		"guild_id":       "guild-001",
		"channel_id":     "channel-001",
		"token":          "interaction-token",
		"timestamp":      "2024-07-15T12:00:00Z",
		"data": map[string]any{
			"id":   "cmd-001",
			"name": "weather",
		},
		"member": map[string]any{
			"user": map[string]any{
				"id":       "user-001",
				"username": "gamer123",
			},
		},
	}

	// Discord sends numeric type, spec maps "2" -> interaction.application_command
	evt, err := Normalize(spec, "2", raw)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if evt.Type != "interaction.application_command" {
		t.Errorf("Type = %q, want %q", evt.Type, "interaction.application_command")
	}
	if evt.ExternalID != "interaction-001" {
		t.Errorf("ExternalID = %q, want %q", evt.ExternalID, "interaction-001")
	}
	if evt.Data["command_name"] != "weather" {
		t.Errorf("Data[command_name] = %v", evt.Data["command_name"])
	}
	if evt.Data["username"] != "gamer123" {
		t.Errorf("Data[username] = %v", evt.Data["username"])
	}
	if evt.Data["guild_id"] != "guild-001" {
		t.Errorf("Data[guild_id] = %v", evt.Data["guild_id"])
	}
}

func TestNormalizeTwilio(t *testing.T) {
	spec := loadSpec(t, "twilio")
	raw := map[string]any{
		"MessageSid":    "SM1234567890",
		"AccountSid":    "AC0987654321",
		"From":          "+15551234567",
		"To":            "+15559876543",
		"Body":          "Hello from Twilio!",
		"MessageStatus": "delivered",
		"SmsStatus":     "delivered",
		"NumMedia":      "0",
	}

	evt, err := Normalize(spec, "delivered", raw)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if evt.Type != "sms.delivered" {
		t.Errorf("Type = %q, want %q", evt.Type, "sms.delivered")
	}
	if evt.ExternalID != "SM1234567890" {
		t.Errorf("ExternalID = %q, want %q", evt.ExternalID, "SM1234567890")
	}
	if evt.Data["from"] != "+15551234567" {
		t.Errorf("Data[from] = %v", evt.Data["from"])
	}
	if evt.Data["to"] != "+15559876543" {
		t.Errorf("Data[to] = %v", evt.Data["to"])
	}
	if evt.Data["body"] != "Hello from Twilio!" {
		t.Errorf("Data[body] = %v", evt.Data["body"])
	}
}

// TestNormalizeAllSpecsWithUnknownEvent ensures every spec gracefully handles
// unmapped event types by passing them through as-is.
func TestNormalizeAllSpecsWithUnknownEvent(t *testing.T) {
	reg := adapters.NewRegistry()
	if err := reg.LoadFS(specs.Embedded); err != nil {
		t.Fatalf("LoadFS: %v", err)
	}

	for _, spec := range reg.List() {
		t.Run(spec.Source, func(t *testing.T) {
			raw := map[string]any{"id": "test-123"}
			evt, err := Normalize(spec, "completely.unknown.event", raw)
			if err != nil {
				t.Fatalf("Normalize: %v", err)
			}
			// Unknown events should pass through as-is
			if evt.Type != "completely.unknown.event" {
				t.Errorf("Type = %q, want passthrough", evt.Type)
			}
			if evt.Source != spec.Source {
				t.Errorf("Source = %q, want %q", evt.Source, spec.Source)
			}
		})
	}
}
