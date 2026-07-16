package cachecore

import (
	"context"
	"errors"
)

// ErrInspectorUnsupported reports that a Store cannot browse cache metadata.
var ErrInspectorUnsupported = errors.New("cache: inspector unsupported for this store")

// Inspector is an optional cache store extension for safe key browsing and metadata inspection.
//
// Not every driver can support this efficiently or at all. Callers should check for support
// with a type assertion and respect Capabilities().
type Inspector interface {
	Capabilities() InspectorCapabilities
	ListPage(ctx context.Context, opts ListPageOptions) (ListPageResult, error)
}

// InspectorCapabilities reports which browsing features a store can support.
type InspectorCapabilities struct {
	// CanList reports whether the store can enumerate cache metadata.
	CanList bool
	// CanRead reports whether listed keys can be read through the Store contract.
	CanRead bool
	// CanDelete reports whether listed keys can be deleted through the Store contract.
	CanDelete bool
	// CanTTL reports whether entries include expiration metadata.
	CanTTL bool
}

// ListPageOptions controls filtering and offset-based pagination for cache inspection.
type ListPageOptions struct {
	// Query filters entries by matching any part of the cache key.
	Query string
	// Prefix is retained as a backward-compatible alias for Query.
	Prefix string
	// Cursor is an opaque continuation value returned by a previous page.
	Cursor string
	// Limit bounds the page size; implementations normalize it to the supported range.
	Limit int
}

// ListPageResult contains one page of cache metadata and its continuation state.
type ListPageResult struct {
	// Entries contains the current page in deterministic key order when the backend supports it.
	Entries []CacheEntry
	// NextCursor continues the listing when HasMore is true.
	NextCursor string
	// HasMore reports whether another page is available.
	HasMore bool
}

// CacheEntry describes one cache item without exposing its value.
type CacheEntry struct {
	// Key is the unprefixed logical cache key.
	Key string
	// SizeBytes is the stored value size reported by the backend.
	SizeBytes int
	// ExpiresAt is the Unix timestamp in milliseconds, or nil when unavailable or non-expiring.
	ExpiresAt *int64
}
