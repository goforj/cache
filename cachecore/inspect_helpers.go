package cachecore

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func NormalizeListLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > 500 {
		return 500
	}
	return limit
}

func DecodeOffsetCursor(cursor string) (int, error) {
	cursor = strings.TrimSpace(cursor)
	if cursor == "" {
		return 0, nil
	}
	offset, err := strconv.Atoi(cursor)
	if err != nil || offset < 0 {
		return 0, fmt.Errorf("invalid cache list cursor %q", cursor)
	}
	return offset, nil
}

func EncodeOffsetCursor(offset int) string {
	if offset <= 0 {
		return ""
	}
	return strconv.Itoa(offset)
}

func ListFilterTerm(opts ListPageOptions) string {
	query := strings.TrimSpace(opts.Query)
	if query != "" {
		return query
	}
	return strings.TrimSpace(opts.Prefix)
}

func FilterAndSortEntries(entries []CacheEntry, filter string) []CacheEntry {
	filter = strings.TrimSpace(filter)
	filtered := make([]CacheEntry, 0, len(entries))
	for _, entry := range entries {
		if filter != "" && !strings.Contains(entry.Key, filter) {
			continue
		}
		filtered = append(filtered, entry)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Key < filtered[j].Key
	})
	return filtered
}

func SliceEntries(entries []CacheEntry, offset int, limit int) ListPageResult {
	limit = NormalizeListLimit(limit)
	if offset < 0 {
		offset = 0
	}
	if offset > len(entries) {
		offset = len(entries)
	}
	end := offset + limit
	if end > len(entries) {
		end = len(entries)
	}
	page := ListPageResult{
		Entries: entries[offset:end],
		HasMore: end < len(entries),
	}
	if page.HasMore {
		page.NextCursor = EncodeOffsetCursor(end)
	}
	return page
}
