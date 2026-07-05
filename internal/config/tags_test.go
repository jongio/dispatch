package config

import (
	"reflect"
	"testing"
)

func TestParseTags(t *testing.T) {
	got := ParseTags(" Work, personal ,,work,PERSONAL ")
	want := []string{"personal", "work"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseTags = %v, want %v", got, want)
	}
	if ParseTags("   ") != nil {
		t.Fatal("expected nil for blank input")
	}
}

func TestSetTagsAndTagsFor(t *testing.T) {
	c := Default()
	c.SetTags("s1", []string{"a", "b"})
	if got := c.TagsFor("s1"); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("TagsFor = %v", got)
	}
	c.SetTags("s1", nil)
	if got := c.TagsFor("s1"); got != nil {
		t.Fatalf("expected nil after clearing, got %v", got)
	}
	c.SetTags("", []string{"x"})
	if len(c.SessionTags) != 0 {
		t.Fatalf("expected no tags for empty id, got %v", c.SessionTags)
	}
}

func TestHasTag(t *testing.T) {
	c := Default()
	c.SetTags("s1", []string{"work", "urgent"})
	if !c.HasTag("s1", "WORK") {
		t.Fatal("expected HasTag to be case-insensitive")
	}
	if c.HasTag("s1", "missing") {
		t.Fatal("expected false for missing tag")
	}
	if c.HasTag("s1", "  ") {
		t.Fatal("expected false for blank tag")
	}
}
