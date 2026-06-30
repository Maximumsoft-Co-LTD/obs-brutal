package boeng_test

import (
	"sort"
	"testing"

	"go.opentelemetry.io/otel/attribute"

	"obs-brutal/boeng"
)

func TestSanitizeMetricName(t *testing.T) {
	cases := map[string]string{
		"create_user":      "create_user",
		"CreateUser":       "createuser",
		"GET /users/:id":   "get_users_id",
		"  weird!! name ":  "weird_name",
		"already_snake_42": "already_snake_42",
		"":                 "unnamed",
		"___":              "unnamed",
		"a":                "a",
	}
	for in, want := range cases {
		if got := boeng.SanitizeMetricNameForTest(in); got != want {
			t.Errorf("sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}

type orderSubject struct {
	OrderID        string // high-cardinality — must be filtered out
	UserID         string // also high-cardinality
	PaymentChannel string // safe — low-cardinality
	UserType       string // safe — low-cardinality
	Region         string // NOT in allowlist for this test — must be filtered
}

func TestMetricLabels_OnlyAllowlistedSubjectKeys(t *testing.T) {
	// Default allowlist is service+env. Add user_type and payment_channel.
	boeng.Init(boeng.Config{
		Service:      "svc",
		Env:          "prod",
		MetricLabels: []string{"user_type", "payment_channel"},
	})

	subj := orderSubject{
		OrderID:        "ord-12345",
		UserID:         "u-67890",
		PaymentChannel: "promptpay",
		UserType:       "premium",
		Region:         "ap-southeast-1",
	}
	got := boeng.MetricLabelsForTest(subj)
	keys := keyset(got)

	wantPresent := []string{"service", "env", "user_type", "payment_channel"}
	for _, k := range wantPresent {
		if !keys[k] {
			t.Errorf("expected label %q present, got %v", k, keys)
		}
	}
	wantAbsent := []string{"order_id", "user_id", "region"}
	for _, k := range wantAbsent {
		if keys[k] {
			t.Errorf("label %q should be filtered (high-cardinality or not allowlisted), got %v", k, keys)
		}
	}
}

func TestMetricLabels_DefaultsOnly(t *testing.T) {
	// No MetricLabels → only service+env should pass through.
	boeng.Init(boeng.Config{Service: "svc", Env: "dev"})

	subj := orderSubject{OrderID: "ord-1", PaymentChannel: "promptpay"}
	got := boeng.MetricLabelsForTest(subj)
	keys := keyset(got)

	if !keys["service"] || !keys["env"] {
		t.Errorf("missing default labels: %v", keys)
	}
	if keys["payment_channel"] {
		t.Errorf("payment_channel leaked without being allowlisted: %v", keys)
	}
	if keys["order_id"] {
		t.Errorf("order_id leaked: %v", keys)
	}
}

func TestMetricLabels_NilSubjectStillEmitsStatics(t *testing.T) {
	boeng.Init(boeng.Config{Service: "svc", Env: "prod"})
	got := boeng.MetricLabelsForTest(nil)
	keys := keyset(got)
	if !keys["service"] || !keys["env"] {
		t.Errorf("nil subject should still carry service+env, got %v", keys)
	}
	if len(keys) != 2 {
		var seen []string
		for k := range keys {
			seen = append(seen, k)
		}
		sort.Strings(seen)
		t.Errorf("expected exactly [env service], got %v", seen)
	}
}

func keyset(kvs []attribute.KeyValue) map[string]bool {
	out := make(map[string]bool, len(kvs))
	for _, kv := range kvs {
		out[string(kv.Key)] = true
	}
	return out
}
