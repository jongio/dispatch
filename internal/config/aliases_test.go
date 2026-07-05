package config

import "testing"

func TestNormalizeAlias(t *testing.T) {
	cases := map[string]string{
		"  AuthFix ": "authfix",
		"my alias":   "my",
		"":           "",
		"UPPER":      "upper",
	}
	for in, want := range cases {
		if got := NormalizeAlias(in); got != want {
			t.Errorf("NormalizeAlias(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSetAliasAndLookup(t *testing.T) {
	c := Default()
	if err := c.SetAlias("s1", "AuthFix"); err != nil {
		t.Fatalf("SetAlias: %v", err)
	}
	if got := c.AliasFor("s1"); got != "authfix" {
		t.Fatalf("AliasFor = %q, want authfix", got)
	}
	if got := c.SessionIDForAlias("AUTHFIX"); got != "s1" {
		t.Fatalf("SessionIDForAlias = %q, want s1", got)
	}
	if got := c.SessionIDForAlias("nope"); got != "" {
		t.Fatalf("expected empty for unknown alias, got %q", got)
	}
}

func TestSetAliasUniqueness(t *testing.T) {
	c := Default()
	if err := c.SetAlias("s1", "dup"); err != nil {
		t.Fatalf("SetAlias s1: %v", err)
	}
	if err := c.SetAlias("s2", "dup"); err == nil {
		t.Fatal("expected uniqueness error for duplicate alias")
	}
	// Re-setting the same alias on the same session is allowed.
	if err := c.SetAlias("s1", "dup"); err != nil {
		t.Fatalf("re-set same alias: %v", err)
	}
}

func TestSetAliasClear(t *testing.T) {
	c := Default()
	_ = c.SetAlias("s1", "temp")
	if err := c.SetAlias("s1", ""); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if got := c.AliasFor("s1"); got != "" {
		t.Fatalf("expected cleared alias, got %q", got)
	}
	if _, ok := c.SessionAliases["s1"]; ok {
		t.Fatal("expected entry removed after clear")
	}
}
