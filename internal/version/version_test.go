package version

import (
	"runtime/debug"
	"testing"
)

func TestCurrent(t *testing.T) {
	original := readBuildInfo
	t.Cleanup(func() { readBuildInfo = original })

	tests := []struct {
		name string
		info *debug.BuildInfo
		ok   bool
		want string
	}{
		{name: "tagged", info: &debug.BuildInfo{Main: debug.Module{Version: "v0.1.0"}}, ok: true, want: "v0.1.0"},
		{name: "development", info: &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, ok: true, want: "dev"},
		{name: "missing", ok: false, want: "dev"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			readBuildInfo = func() (*debug.BuildInfo, bool) { return test.info, test.ok }
			if got := Current(); got != test.want {
				t.Fatalf("Current() = %q, want %q", got, test.want)
			}
		})
	}
}
