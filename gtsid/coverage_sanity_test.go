package gtsid

import "testing"

func mustPat(t *testing.T, s string) *ID {
	t.Helper()
	p, err := ParsePattern(s)
	if err != nil {
		t.Fatalf("ParsePattern(%q): %v", s, err)
	}
	return p
}

func TestCoverageSanity(t *testing.T) {
	cases := []struct {
		operand string
		value   string
		want    bool
	}{
		// ADR-0007 resource-family scope: operand gts.x.core.resources.resource.*
		{"gts.x.core.resources.resource.*", "gts.x.core.resources.resource.v1~", true},
		{"gts.x.core.resources.resource.*", "gts.x.core.resources.resource.v1~acme.storage.resources.bucket.*", true},
		{"gts.x.core.resources.resource.*", "gts.x.core.resources.*", false},
		{"gts.x.core.resources.resource.*", "gts.x.core.events.event.*", false},
		// README §10 minor-version flexibility.
		{"gts.x.core.srr.resource.v1~*", "gts.x.core.srr.resource.v1.0~*", true},
		{"gts.x.core.srr.resource.v1~", "gts.x.core.srr.resource.v1.0~", true},
		{"gts.x.core.srr.resource.v1~*", "gts.x.core.srr.resource.v1~acme.*", true},
		{"gts.x.core.srr.resource.v1~acme.*", "gts.x.core.srr.resource.v1~*", false},
		// gts.* covers everything.
		{"gts.*", "gts.x.core.resources.resource.*", true},
		{"gts.*", "gts.x.core.events.type.v1~x.commerce.orders.order_placed.v1.0", true},
	}
	for _, c := range cases {
		got := Covers(mustPat(t, c.operand), mustPat(t, c.value))
		if got != c.want {
			t.Errorf("Covers(%q, %q) = %v, want %v", c.operand, c.value, got, c.want)
		}
	}
}

func TestMatchesPatternSanity(t *testing.T) {
	cases := []struct {
		pattern   string
		candidate string
		want      bool
	}{
		{"gts.x.infra.compute.vm_state.v1~", "gts.x.infra.compute.vm_state.v1~", true},
		{"gts.x.infra.compute.vm_state.v1~", "gts.x.infra.compute.vm_state.v1~x.infra._.running.v1", true},
		{"gts.x.infra.compute.vm_state.v1~", "gts.x.infra.compute.vm_state.v2~", false},
		{"gts.x.infra.compute.vm_state.v1~*", "gts.x.infra.compute.vm_state.v1~", true},
		{"gts.*", "gts.x.infra.compute.vm_state.v1~", true},
	}
	for _, c := range cases {
		cand, err := New(c.candidate)
		if err != nil {
			t.Fatalf("New(%q): %v", c.candidate, err)
		}
		got := MatchesPattern(cand, mustPat(t, c.pattern))
		if got != c.want {
			t.Errorf("MatchesPattern(%q, %q) = %v, want %v", c.candidate, c.pattern, got, c.want)
		}
	}
}
