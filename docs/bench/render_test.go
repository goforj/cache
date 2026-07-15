//go:build benchrender
// +build benchrender

package bench

import "testing"

// TestRenderBenchmarks verifies the benchmark report renders from the checked-in results.
func TestRenderBenchmarks(t *testing.T) {
	RenderBenchmarks()
}
