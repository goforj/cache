package cache

import (
	"context"
	"errors"

	"github.com/goforj/cache/cachecore"
)

var ErrInspectorUnsupported = errors.New("cache: inspector unsupported for this store")

func (c *Cache) Inspector() (cachecore.Inspector, bool) {
	if c == nil || c.store == nil {
		return nil, false
	}
	inspector, ok := c.store.(cachecore.Inspector)
	return inspector, ok
}

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
