package routes

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/mcpx/boilerplate/stores"
)

func TestBuildAdminUserResponseJSONKeys(t *testing.T) {
	now := time.Date(2025, 10, 20, 0, 0, 0, 0, time.UTC)
	user := &stores.UserModel{
		ID:        1,
		Email:     "person@example.com",
		Name:      "Person",
		AvatarURL: "https://example.com/avatar.png",
		Role:      stores.UserRoleAdmin,
		Status:    stores.UserStatusActive,
		CreatedAt: now,
	}

	payload := buildAdminUserResponse(user)
	keys := jsonKeys(t, payload)

	assertHasKey(t, keys, "id")
	assertHasKey(t, keys, "email")
	assertHasKey(t, keys, "name")
	assertHasKey(t, keys, "avatar")
	assertHasKey(t, keys, "role")
	assertHasKey(t, keys, "status")
	assertHasKey(t, keys, "created_at")
}

func TestBuildAdminUserMetricsResponseJSONKeys(t *testing.T) {
	now := time.Date(2025, 10, 20, 0, 0, 0, 0, time.UTC)
	metrics := &stores.UserMetricsModel{
		UserID:       9,
		Score:        42,
		Stage:        stores.FunnelStageEngaged,
		LastActiveAt: &now,
		Reads14d:     3,
		Completes14d: 1,
		UpdatedAt:    now,
	}

	payload := buildAdminUserMetricsResponse(metrics)
	if payload == nil {
		t.Fatal("expected metrics response")
	}
	keys := jsonKeys(t, payload)

	assertHasKey(t, keys, "user_id")
	assertHasKey(t, keys, "score")
	assertHasKey(t, keys, "stage")
	assertHasKey(t, keys, "last_active_at")
	assertHasKey(t, keys, "reads_14d")
	assertHasKey(t, keys, "completes_14d")
	assertHasKey(t, keys, "updated_at")
}

func TestBuildAdminEntitlementResponseJSONKeys(t *testing.T) {
	now := time.Date(2025, 10, 20, 0, 0, 0, 0, time.UTC)
	entitlement := &stores.EntitlementModel{
		ID:            22,
		UserID:        9,
		PlanCode:      "YEARLY",
		Status:        stores.EntitlementStatusActive,
		StartAt:       now,
		EndAt:         now.AddDate(0, 1, 0),
		LastPaymentID: 5,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	payload := buildAdminEntitlementResponse(entitlement)
	if payload == nil {
		t.Fatal("expected entitlement response")
	}
	keys := jsonKeys(t, payload)

	assertHasKey(t, keys, "id")
	assertHasKey(t, keys, "user_id")
	assertHasKey(t, keys, "plan_code")
	assertHasKey(t, keys, "status")
	assertHasKey(t, keys, "start_at")
	assertHasKey(t, keys, "end_at")
	assertHasKey(t, keys, "last_payment_id")
}

func TestBuildAdminUserMetricsResponseNil(t *testing.T) {
	if buildAdminUserMetricsResponse(nil) != nil {
		t.Fatal("expected nil metrics response")
	}
}

func TestBuildAdminEntitlementResponseNil(t *testing.T) {
	if buildAdminEntitlementResponse(nil) != nil {
		t.Fatal("expected nil entitlement response")
	}
}

func jsonKeys(t *testing.T, payload interface{}) map[string]interface{} {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	out := map[string]interface{}{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}
	return out
}

func assertHasKey(t *testing.T, payload map[string]interface{}, key string) {
	t.Helper()
	if _, ok := payload[key]; !ok {
		t.Fatalf("expected key %q", key)
	}
}
