package integration_test

import (
	"testing"
)

// TestSketchAcrossLanguages runs sketch on PaymentService across all language fixtures.
// Ensures core command behavior is not TypeScript-specific.
func TestSketchAcrossLanguages(t *testing.T) {
	fixtures := []string{"typescript-simple", "python-simple", "rust-simple", "csharp-simple", "go-simple", "java-simple", "kotlin-simple", "ruby-simple", "php-simple", "swift-simple"}
	for _, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			dir := setupIndexedFixture(t, fixture)
			stdout, _ := runInari(t, dir, "sketch", "PaymentService")
			assertContains(t, stdout, "PaymentService")
		})
	}
}

// TestFindAcrossLanguages runs find on PaymentService across all language fixtures.
func TestFindAcrossLanguages(t *testing.T) {
	fixtures := []string{"typescript-simple", "python-simple", "rust-simple", "csharp-simple", "go-simple", "java-simple", "kotlin-simple", "ruby-simple", "php-simple", "swift-simple"}
	for _, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			dir := setupIndexedFixture(t, fixture)
			stdout, _ := runInari(t, dir, "find", "PaymentService", "--json")
			envelope := parseJSON(t, stdout)
			data, ok := envelope["data"].([]interface{})
			if !ok {
				t.Fatalf("expected data to be an array, got %T", envelope["data"])
			}
			if len(data) == 0 {
				t.Error("expected PaymentService to be found")
			}
		})
	}
}

// TestRefsAcrossLanguages runs refs on process_payment/ProcessPayment across fixtures.
func TestRefsAcrossLanguages(t *testing.T) {
	cases := []struct {
		fixture string
		symbol  string
	}{
		{"typescript-simple", "processPayment"},
		{"python-simple", "process_payment"},
		{"rust-simple", "process_payment"},
		{"csharp-simple", "ProcessPayment"},
		{"go-simple", "ProcessPayment"},
		{"java-simple", "processPayment"},
		{"kotlin-simple", "processPayment"},
		{"ruby-simple", "process_payment"},
		{"php-simple", "processPayment"},
		{"lua-simple", "processPayment"},
		{"swift-simple", "processPayment"},
		{"bash-simple", "process_payment"},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			dir := setupIndexedFixture(t, tc.fixture)
			runInari(t, dir, "refs", tc.symbol)
		})
	}
}

// TestMapAcrossLanguages runs map on all language fixtures.
func TestMapAcrossLanguages(t *testing.T) {
	cases := []struct {
		fixture  string
		contains string
	}{
		{"typescript-simple", "PaymentService"},
		{"python-simple", "PaymentService"},
		{"rust-simple", "process_payment"},
		{"csharp-simple", "PaymentService"},
		{"go-simple", "ProcessPayment"},
		{"java-simple", "PaymentService"},
		{"kotlin-simple", "PaymentService"},
		{"ruby-simple", "PaymentService"},
		{"php-simple", "PaymentService"},
		{"lua-simple", "processPayment"},
		{"swift-simple", "PaymentService"},
		{"bash-simple", "process_payment"},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			dir := setupIndexedFixture(t, tc.fixture)
			stdout, _ := runInari(t, dir, "map")
			assertContains(t, stdout, tc.contains)
		})
	}
}
