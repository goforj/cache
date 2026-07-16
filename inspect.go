package cache

import (
	"context"

	"github.com/goforj/cache/cachecore"
)

// ErrInspectorUnsupported reports that a Store cannot browse cache metadata.
var ErrInspectorUnsupported = cachecore.ErrInspectorUnsupported

// Inspector returns the optional browsing interface for the underlying store.
// @group Core
//
// Example: detect inspector support
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	inspector, ok := c.Inspector()
//	fmt.Println(ok, inspector.Capabilities().CanList) // true true
func (c *Cache) Inspector() (cachecore.Inspector, bool) {
	if c == nil || c.store == nil {
		return nil, false
	}
	inspector, ok := c.store.(cachecore.Inspector)
	return inspector, ok
}

// ListPage lists cache entries from an inspector-capable store.
// @group Reads
//
// Example: browse cache keys
//
//	ctx := context.Background()
//	c := cache.NewCache(cache.NewMemoryStore(ctx))
//	_ = c.SetString("profile:1", "Ada", time.Minute)
//	_ = c.SetString("profile:2", "Grace", time.Minute)
//	page, _ := cache.ListPage(ctx, c, cachecore.ListPageOptions{
//		Query: "profile:",
//		Limit: 10,
//	})
//	fmt.Println(len(page.Entries), page.Entries[0].Key) // 2 profile:1
func ListPage(ctx context.Context, c *Cache, opts cachecore.ListPageOptions) (cachecore.ListPageResult, error) {
	if c == nil {
		return cachecore.ListPageResult{}, ErrInspectorUnsupported
	}
	inspector, ok := c.Inspector()
	if !ok {
		return cachecore.ListPageResult{}, ErrInspectorUnsupported
	}
	return inspector.ListPage(ctx, opts)
}
