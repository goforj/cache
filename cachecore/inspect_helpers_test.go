package cachecore

import (
	"reflect"
	"testing"
)

// TestNormalizeListLimit verifies defaulting, passthrough, and safety-cap behavior.
func TestNormalizeListLimit(t *testing.T) {
	tests := []struct {
		name  string
		limit int
		want  int
	}{
		{name: "negative", limit: -1, want: 100},
		{name: "zero", limit: 0, want: 100},
		{name: "requested", limit: 25, want: 25},
		{name: "capped", limit: 501, want: 500},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := NormalizeListLimit(test.limit); got != test.want {
				t.Fatalf("NormalizeListLimit(%d) = %d, want %d", test.limit, got, test.want)
			}
		})
	}
}

// TestOffsetCursor verifies valid cursor round trips and malformed cursor rejection.
func TestOffsetCursor(t *testing.T) {
	for _, test := range []struct {
		cursor string
		want   int
	}{
		{cursor: "", want: 0},
		{cursor: "  ", want: 0},
		{cursor: " 42 ", want: 42},
	} {
		got, err := DecodeOffsetCursor(test.cursor)
		if err != nil || got != test.want {
			t.Fatalf("DecodeOffsetCursor(%q) = %d, %v; want %d, nil", test.cursor, got, err, test.want)
		}
	}
	for _, cursor := range []string{"-1", "not-a-number"} {
		if _, err := DecodeOffsetCursor(cursor); err == nil {
			t.Fatalf("DecodeOffsetCursor(%q) returned nil error", cursor)
		}
	}
	if got := EncodeOffsetCursor(0); got != "" {
		t.Fatalf("EncodeOffsetCursor(0) = %q, want empty", got)
	}
	if got := EncodeOffsetCursor(-1); got != "" {
		t.Fatalf("EncodeOffsetCursor(-1) = %q, want empty", got)
	}
	if got := EncodeOffsetCursor(42); got != "42" {
		t.Fatalf("EncodeOffsetCursor(42) = %q, want 42", got)
	}
}

// TestListFilterTerm verifies Query precedence and the compatibility Prefix fallback.
func TestListFilterTerm(t *testing.T) {
	if got := ListFilterTerm(ListPageOptions{Query: " query ", Prefix: "prefix"}); got != "query" {
		t.Fatalf("query term = %q, want query", got)
	}
	if got := ListFilterTerm(ListPageOptions{Query: " ", Prefix: " prefix "}); got != "prefix" {
		t.Fatalf("prefix term = %q, want prefix", got)
	}
}

// TestFilterAndSliceEntries verifies deterministic filtering and pagination boundaries.
func TestFilterAndSliceEntries(t *testing.T) {
	entries := []CacheEntry{
		{Key: "page/c"},
		{Key: "other/z"},
		{Key: "page/a"},
		{Key: "page/b"},
	}
	filtered := FilterAndSortEntries(entries, " page/ ")
	wantFiltered := []CacheEntry{{Key: "page/a"}, {Key: "page/b"}, {Key: "page/c"}}
	if !reflect.DeepEqual(filtered, wantFiltered) {
		t.Fatalf("filtered entries = %+v, want %+v", filtered, wantFiltered)
	}
	if got := FilterAndSortEntries(entries, ""); len(got) != len(entries) || got[0].Key != "other/z" {
		t.Fatalf("unfiltered entries were not sorted: %+v", got)
	}

	first := SliceEntries(filtered, -1, 2)
	if !reflect.DeepEqual(first.Entries, wantFiltered[:2]) || !first.HasMore || first.NextCursor != "2" {
		t.Fatalf("first page = %+v", first)
	}
	last := SliceEntries(filtered, 2, 10)
	if !reflect.DeepEqual(last.Entries, wantFiltered[2:]) || last.HasMore || last.NextCursor != "" {
		t.Fatalf("last page = %+v", last)
	}
	empty := SliceEntries(filtered, 99, 10)
	if len(empty.Entries) != 0 || empty.HasMore || empty.NextCursor != "" {
		t.Fatalf("out-of-range page = %+v", empty)
	}
}
