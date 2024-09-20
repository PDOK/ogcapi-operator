package controller

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_createIngressRuleMatchFromURL(t *testing.T) {
	type args struct {
		url                     url.URL
		includeLocalhost        bool
		matchUnderscoreVersions bool
	}
	tests := []struct {
		name string
		args args
		want string
	}{{
		name: "no path",
		args: args{
			url:                     mustURLParse(t, "http://host.my"),
			includeLocalhost:        false,
			matchUnderscoreVersions: false,
		},
		want: "Host(`host.my`) && PathPrefix(``)",
	}, {
		name: "include localhost",
		args: args{
			url:                     mustURLParse(t, "http://host.my"),
			includeLocalhost:        true,
			matchUnderscoreVersions: false,
		},
		want: "(Host(`localhost`) || Host(`host.my`)) && PathPrefix(``)",
	}, {
		name: "no versions",
		args: args{
			url:                     mustURLParse(t, "http://host.my/some/path"),
			includeLocalhost:        false,
			matchUnderscoreVersions: false,
		},
		want: "Host(`host.my`) && PathPrefix(`/some/path`)",
	}, {
		name: "v1, no replacing",
		args: args{
			url:                     mustURLParse(t, "http://host.my/some/path/v1"),
			includeLocalhost:        false,
			matchUnderscoreVersions: false,
		},
		want: "Host(`host.my`) && PathPrefix(`/some/path/v1`)",
	}, {
		name: "v1",
		args: args{
			url:                     mustURLParse(t, "http://host.my/some/path/v1"),
			includeLocalhost:        false,
			matchUnderscoreVersions: true,
		},
		want: "Host(`host.my`) && PathRegexp(`^/some/path/v1(_\\d+)?$`)",
	}, {
		name: "v2, followed by other segment",
		args: args{
			url:                     mustURLParse(t, "http://host.my/some/path/v2/foo"),
			includeLocalhost:        false,
			matchUnderscoreVersions: true,
		},
		want: "Host(`host.my`) && PathRegexp(`^/some/path/v2(_\\d+)?/foo$`)",
	}, {
		name: "v1_3",
		args: args{
			url:                     mustURLParse(t, "http://host.my/some/path/v1_3"),
			includeLocalhost:        false,
			matchUnderscoreVersions: true,
		},
		want: "Host(`host.my`) && PathRegexp(`^/some/path/v1(_3)?$`)",
	}, {
		name: "combined (never happens)",
		args: args{
			url:                     mustURLParse(t, "http://host.my/some/path/v345/v666_78"),
			includeLocalhost:        false,
			matchUnderscoreVersions: true,
		},
		want: "Host(`host.my`) && PathRegexp(`^/some/path/v345(_\\d+)?/v666(_78)?$`)",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := createIngressRuleMatchFromURL(tt.args.url, tt.args.includeLocalhost, tt.args.matchUnderscoreVersions); got != tt.want {
				t.Errorf("createIngressRuleMatchFromURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func mustURLParse(t *testing.T, in string) url.URL {
	parsed, err := url.Parse(in)
	require.NoError(t, err)
	return *parsed
}
