package rekvizitupaieska

import (
	"runtime/debug"
	"testing"
)

func TestLibraryVersionFromBuildInfo(t *testing.T) {
	cases := []struct {
		name string
		info *debug.BuildInfo
		want string
	}{
		{name: "no build info", info: nil, want: develVersion},
		{
			name: "own tests from a checkout",
			info: &debug.BuildInfo{Main: debug.Module{Path: modulePath, Version: goDevelVersion}},
			want: develVersion,
		},
		{
			name: "main module without a version",
			info: &debug.BuildInfo{Main: debug.Module{Path: modulePath}},
			want: develVersion,
		},
		{
			name: "main module at a tag",
			info: &debug.BuildInfo{Main: debug.Module{Path: modulePath, Version: "v0.1.0"}},
			want: "v0.1.0",
		},
		{
			name: "dependency of a caller's binary",
			info: &debug.BuildInfo{
				Main: debug.Module{Path: "example.com/caller", Version: goDevelVersion},
				Deps: []*debug.Module{
					{Path: "github.com/PuerkitoBio/goquery", Version: "v1.12.0"},
					{Path: modulePath, Version: "v0.1.0"},
				},
			},
			want: "v0.1.0",
		},
		{
			name: "replaced dependency reports the replacement",
			info: &debug.BuildInfo{
				Main: debug.Module{Path: "example.com/caller"},
				Deps: []*debug.Module{
					{Path: modulePath, Version: "v0.1.0", Replace: &debug.Module{Path: "example.com/fork", Version: "v0.1.1"}},
				},
			},
			want: "v0.1.1",
		},
		{
			name: "dependency replaced by a local directory",
			info: &debug.BuildInfo{
				Main: debug.Module{Path: "example.com/caller"},
				Deps: []*debug.Module{
					{Path: modulePath, Version: "v0.1.0", Replace: &debug.Module{Path: "../rekvizitupaieska"}},
				},
			},
			want: develVersion,
		},
		{
			name: "module absent",
			info: &debug.BuildInfo{
				Main: debug.Module{Path: "example.com/caller"},
				Deps: []*debug.Module{{Path: "github.com/PuerkitoBio/goquery", Version: "v1.12.0"}},
			},
			want: develVersion,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := libraryVersionFromBuildInfo(testCase.info)
			if got != testCase.want {
				t.Errorf("libraryVersionFromBuildInfo = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestBuildUserAgentNamesTheLibraryAndItsVersion(t *testing.T) {
	want := "rekvizitupaieska/v0.1.0 (+https://github.com/pauliusgin/rekvizitupaieska)"

	got := buildUserAgent("v0.1.0")

	if got != want {
		t.Errorf("buildUserAgent = %q, want %q", got, want)
	}
}
