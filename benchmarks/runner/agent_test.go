package runner

import (
	"testing"
)

func TestAccumulateUsage(t *testing.T) {
	var input, output, cacheCreation, cacheRead uint64

	value := map[string]interface{}{
		"usage": map[string]interface{}{
			"input_tokens":                float64(100),
			"output_tokens":               float64(50),
			"cache_creation_input_tokens": float64(200),
			"cache_read_input_tokens":     float64(300),
		},
	}

	accumulateUsage(value, &input, &output, &cacheCreation, &cacheRead)

	if input != 100 {
		t.Errorf("expected input_tokens=100, got %d", input)
	}
	if output != 50 {
		t.Errorf("expected output_tokens=50, got %d", output)
	}
	if cacheCreation != 200 {
		t.Errorf("expected cache_creation=200, got %d", cacheCreation)
	}
	if cacheRead != 300 {
		t.Errorf("expected cache_read=300, got %d", cacheRead)
	}
}

func TestAccumulateUsageNoUsageField(t *testing.T) {
	var input, output, cacheCreation, cacheRead uint64

	value := map[string]interface{}{
		"type": "message",
	}

	accumulateUsage(value, &input, &output, &cacheCreation, &cacheRead)

	if input != 0 || output != 0 || cacheCreation != 0 || cacheRead != 0 {
		t.Error("expected all counters to remain zero when no usage field present")
	}
}

func TestGetStringField(t *testing.T) {
	m := map[string]interface{}{
		"type": "result",
		"num":  float64(42),
	}

	if got := getStringField(m, "type"); got != "result" {
		t.Errorf("expected 'result', got '%s'", got)
	}
	if got := getStringField(m, "missing"); got != "" {
		t.Errorf("expected empty string for missing key, got '%s'", got)
	}
	if got := getStringField(m, "num"); got != "" {
		t.Errorf("expected empty string for non-string value, got '%s'", got)
	}
}

func TestGetUint64Field(t *testing.T) {
	m := map[string]interface{}{
		"tokens": float64(12345),
		"text":   "hello",
	}

	if got := getUint64Field(m, "tokens"); got != 12345 {
		t.Errorf("expected 12345, got %d", got)
	}
	if got := getUint64Field(m, "missing"); got != 0 {
		t.Errorf("expected 0 for missing key, got %d", got)
	}
	if got := getUint64Field(m, "text"); got != 0 {
		t.Errorf("expected 0 for non-numeric value, got %d", got)
	}
}

func TestContainsString(t *testing.T) {
	slice := []string{"sketch", "map", "refs"}

	if !containsString(slice, "map") {
		t.Error("expected to find 'map' in slice")
	}
	if containsString(slice, "deps") {
		t.Error("expected 'deps' to not be in slice")
	}
	if containsString(nil, "anything") {
		t.Error("expected nil slice to not contain anything")
	}
}

func TestExtractInariCommand(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"inari sketch PaymentService", "inari sketch"},
		{"inari map", "inari map"},
		{"inari refs processPayment --json", "inari refs"},
		{"ls -la", ""},
		{"grep inari README.md", "inari README.md"}, // false positive is acceptable
		{"bin/inari index --full", "inari index"},
	}

	for _, tc := range cases {
		got := extractInariCommand(tc.input)
		if got != tc.expected {
			t.Errorf("extractInariCommand(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}
