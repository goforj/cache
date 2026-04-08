package cachecore

import "context"

// Inspector is an optional cache store extension for safe key browsing and metadata inspection.
//
// Not every driver can support this efficiently or at all. Callers should check for support
// with a type assertion and respect Capabilities().
type Inspector interface {
	Capabilities() InspectorCapabilities
	ListPage(ctx context.Context, opts ListPageOptions) (ListPageResult, error)
}

type InspectorCapabilities struct {
	CanList   bool
	CanRead   bool
	CanDelete bool
	CanTTL    bool
}

type ListPageOptions struct {
	// Query filters entries by matching any part of the cache key.
	// Prefix is retained as a backward-compatible alias.
	Query  string
	Prefix string
	Cursor string
	Limit  int
}

type ListPageResult struct {
	Entries    []CacheEntry
	NextCursor string
	HasMore    bool
}

type CacheEntry struct {
	Key       string
	SizeBytes int
	ExpiresAt *int64
}
