package services

import (
	"testing"

	"github.com/ablate-ai/RuleFlow/database"
)

func TestPolicyReferencesSubscription(t *testing.T) {
	policy := &database.ConfigPolicy{
		ID:              1,
		Token:           "demo-token",
		SubscriptionIDs: []int64{1001, 1002, 1003},
	}

	if !policyReferencesSubscription(policy, 1002) {
		t.Fatalf("期望命中关联订阅")
	}

	if policyReferencesSubscription(policy, 2001) {
		t.Fatalf("不期望命中未关联订阅")
	}

	if policyReferencesSubscription(nil, 1002) {
		t.Fatalf("nil 策略不应命中")
	}
}
