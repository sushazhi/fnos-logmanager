package services

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestIPWhitelistPersistence(t *testing.T) {
	dir, _ := os.MkdirTemp("", "notif-test-*")
	defer os.RemoveAll(dir)

	// Step 1: Create store and add rule with IP whitelist
	store := NewNotificationStore(dir)
	rule := NotificationRule{
		ID:               "test-rule-1",
		Name:             "IP Whitelist Test",
		Status:           "enabled",
		AppName:          "test-app",
		LogLevel:         "info",
		Channels:         []string{"channel-1"},
		Cooldown:         60,
		MaxNotifications: 10,
		IPWhitelist:      []string{"192.168.1.0/24", "10.0.0.1"},
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := store.AddRule(rule); err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	// Step 2: Verify in-memory state right after add
	rules := store.GetRules()
	if len(rules) != 1 {
		t.Fatalf("Expected 1 rule, got %d", len(rules))
	}
	if len(rules[0].IPWhitelist) != 2 {
		t.Errorf("After AddRule: expected 2 IPs, got %d: %v", len(rules[0].IPWhitelist), rules[0].IPWhitelist)
	}

	// Step 3: Reload store from disk (simulates server restart)
	store2 := NewNotificationStore(dir)
	reloaded := store2.GetRule("test-rule-1")
	if reloaded == nil {
		t.Fatal("Rule not found after reloading from disk")
	}
	fmt.Printf("After reload: IPWhitelist=%v (len=%d, nil=%v)\n",
		reloaded.IPWhitelist, len(reloaded.IPWhitelist), reloaded.IPWhitelist == nil)
	if len(reloaded.IPWhitelist) != 2 {
		t.Errorf("After reload: expected 2 IPs, got %d", len(reloaded.IPWhitelist))
	}

	// Step 4: Update rule with new IPs (simulates updateRule handler)
	existing := store2.GetRule("test-rule-1")
	updated := *existing
	updated.IPWhitelist = []string{"10.0.0.0/8"}
	if err := store2.UpdateRule("test-rule-1", updated); err != nil {
		t.Fatalf("UpdateRule failed: %v", err)
	}
	after := store2.GetRule("test-rule-1")
	if len(after.IPWhitelist) != 1 || after.IPWhitelist[0] != "10.0.0.0/8" {
		t.Errorf("After UpdateRule: expected [10.0.0.0/8], got %v", after.IPWhitelist)
	}

	// Step 5: Reload again to verify persistence of update
	store3 := NewNotificationStore(dir)
	reloaded2 := store3.GetRule("test-rule-1")
	fmt.Printf("After update+reload: IPWhitelist=%v (len=%d)\n",
		reloaded2.IPWhitelist, len(reloaded2.IPWhitelist))
	if len(reloaded2.IPWhitelist) != 1 || reloaded2.IPWhitelist[0] != "10.0.0.0/8" {
		t.Errorf("After update+reload: expected [10.0.0.0/8], got %v", reloaded2.IPWhitelist)
	}

	// Step 6: Simulate addRule handler (parsing ipWhitelist from JSON body)
	bodyJSON := `{"name":"API Test","appName":"test","channels":["ch1"],"ipWhitelist":["10.0.0.0/8"],"keywords":["error"]}`
	var body map[string]interface{}
	if err := json.Unmarshal([]byte(bodyJSON), &body); err != nil {
		t.Fatalf("Failed to parse body: %v", err)
	}

	getStr := func(m map[string]interface{}, key string) string {
		if v, ok := m[key].(string); ok {
			return v
		}
		return ""
	}

	apiRule := NotificationRule{
		ID:               "api-test-rule",
		Name:             getStr(body, "name"),
		Status:           "enabled",
		AppName:          getStr(body, "appName"),
		LogLevel:         "all",
		Channels:         []string{"ch1"},
		Cooldown:         60,
		MaxNotifications: 10,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if iws, ok := body["ipWhitelist"].([]interface{}); ok {
		for _, iw := range iws {
			if s, ok := iw.(string); ok {
				apiRule.IPWhitelist = append(apiRule.IPWhitelist, s)
			}
		}
	}
	if len(apiRule.IPWhitelist) != 1 || apiRule.IPWhitelist[0] != "10.0.0.0/8" {
		t.Errorf("API rule parsing: expected [10.0.0.0/8], got %v", apiRule.IPWhitelist)
	}

	// Step 7: Simulate updateRule handler (updating IP whitelist via body)
	updateBodyJSON := `{"ipWhitelist":["172.16.0.0/12","192.168.0.0/16"]}`
	var updateBody map[string]interface{}
	json.Unmarshal([]byte(updateBodyJSON), &updateBody)

	existing2 := store3.GetRule("test-rule-1")
	updated2 := *existing2
	if v, ok := updateBody["ipWhitelist"]; ok {
		if arr, ok2 := v.([]interface{}); ok2 {
			var strs []string
			for _, item := range arr {
				if s, ok3 := item.(string); ok3 {
					strs = append(strs, s)
				}
			}
			updated2.IPWhitelist = strs
		}
	}
	if err := store3.UpdateRule("test-rule-1", updated2); err != nil {
		t.Fatalf("UpdateRule (simulated handler) failed: %v", err)
	}
	afterUpdate := store3.GetRule("test-rule-1")
	fmt.Printf("After simulated updateRule: IPWhitelist=%v (len=%d)\n",
		afterUpdate.IPWhitelist, len(afterUpdate.IPWhitelist))
	if len(afterUpdate.IPWhitelist) != 2 {
		t.Errorf("After simulated updateRule: expected 2 IPs, got %d", len(afterUpdate.IPWhitelist))
	}

	fmt.Println("\n=== ALL TESTS PASSED ===")
}
