package cachetest

import (
	"context"
	"testing"
	"time"

	"github.com/goforj/cache/cachecore"
)

// RunInspectorContract runs a backend-agnostic inspector suite for stores that support cache browsing.
func RunInspectorContract(t *testing.T, store cachecore.Store) {
	t.Helper()

	inspector, ok := store.(cachecore.Inspector)
	if !ok {
		t.Fatalf("store %T does not implement cachecore.Inspector", store)
	}
	if caps := inspector.Capabilities(); !caps.CanList {
		t.Fatalf("expected CanList capability")
	}

	ctx := context.Background()
	for _, item := range []struct {
		key   string
		value string
	}{
		{key: "page/a", value: "alpha"},
		{key: "page/b", value: "bravo"},
		{key: "page/c", value: "charlie"},
		{key: "other/z", value: "zulu"},
	} {
		if err := store.Set(ctx, item.key, []byte(item.value), time.Minute); err != nil {
			t.Fatalf("set %s failed: %v", item.key, err)
		}
	}

	first, err := inspector.ListPage(ctx, cachecore.ListPageOptions{Query: "page/", Limit: 2})
	if err != nil {
		t.Fatalf("first page failed: %v", err)
	}
	if len(first.Entries) != 2 || first.Entries[0].Key != "page/a" || first.Entries[1].Key != "page/b" {
		t.Fatalf("unexpected first page: %+v", first.Entries)
	}
	if !first.HasMore || first.NextCursor == "" {
		t.Fatalf("expected next cursor on first page: %+v", first)
	}
	if first.Entries[0].SizeBytes <= 0 {
		t.Fatalf("expected size metadata on first entry")
	}

	second, err := inspector.ListPage(ctx, cachecore.ListPageOptions{
		Query:  "page/",
		Limit:  2,
		Cursor: first.NextCursor,
	})
	if err != nil {
		t.Fatalf("second page failed: %v", err)
	}
	if len(second.Entries) != 1 || second.Entries[0].Key != "page/c" {
		t.Fatalf("unexpected second page: %+v", second.Entries)
	}
	if second.HasMore || second.NextCursor != "" {
		t.Fatalf("expected terminal page: %+v", second)
	}

	substring, err := inspector.ListPage(ctx, cachecore.ListPageOptions{Query: "/b", Limit: 10})
	if err != nil {
		t.Fatalf("substring page failed: %v", err)
	}
	if len(substring.Entries) != 1 || substring.Entries[0].Key != "page/b" {
		t.Fatalf("unexpected substring page: %+v", substring.Entries)
	}
}
