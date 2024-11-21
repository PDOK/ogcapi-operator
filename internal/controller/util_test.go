package controller

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_createIngressRuleAndStripPrefixForURL(t *testing.T) {
	type args struct {
		url                     url.URL
		includelocalhost        bool
		matchUnderscoreVersions bool
		traefikV2               bool
	}
	type wants struct {
		rule   string
		prefix string
	}
	tests := []struct {
		name  string
		args  args
		wants wants
	}{{
		name: "no path",
		args: args{
			url: mustURLParse(t, "http://example.com"),
		},
		wants: wants{
			rule:   "Host(`example.com`)",
			prefix: "",
		},
	}, {
		name: "include localhost",
		args: args{
			url:              mustURLParse(t, "http://example.com"),
			includelocalhost: true,
		},
		wants: wants{
			rule:   "(Host(`localhost`) || Host(`example.com`))",
			prefix: ""},
	}, {
		name: "no versions",
		args: args{
			url: mustURLParse(t, "http://example.com/some/path"),
		},
		wants: wants{
			rule:   "Host(`example.com`) && PathRegexp(`^/some/path(/|$)`)",
			prefix: "^/some/path"},
	}, {
		name: "v1, no replacing",
		args: args{
			url:                     mustURLParse(t, "http://example.com/some/path/v1"),
			matchUnderscoreVersions: false,
		},
		wants: wants{
			rule:   "Host(`example.com`) && PathRegexp(`^/some/path/v1(/|$)`)",
			prefix: "^/some/path/v1"},
	}, {
		name: "v1, no replacing, trailing slash",
		args: args{
			url:                     mustURLParse(t, "http://example.com/some/path/v1/"),
			matchUnderscoreVersions: false,
		},
		wants: wants{
			rule:   "Host(`example.com`) && PathRegexp(`^/some/path/v1/`)",
			prefix: "^/some/path/v1/"},
	}, {
		name: "v1",
		args: args{
			url:                     mustURLParse(t, "http://example.com/some/path/v1"),
			matchUnderscoreVersions: true,
		},
		wants: wants{
			rule:   "Host(`example.com`) && PathRegexp(`^/some/path/v1(_\\d+)?(/|$)`)",
			prefix: "^/some/path/v1(_\\d+)?"},
	}, {
		name: "v1, for traefikV2",
		args: args{
			url:                     mustURLParse(t, "http://example.com/some/path/v1"),
			matchUnderscoreVersions: true,
			traefikV2:               true,
		},
		wants: wants{
			rule:   "Host(`example.com`) && PathPrefix(`/{path:some/path/v1(_\\d+)?(/|$)}`)",
			prefix: "^/some/path/v1(_\\d+)?"},
	}, {
		name: "v2, followed by other segment",
		args: args{
			url:                     mustURLParse(t, "http://example.com/some/path/v2/foo"),
			matchUnderscoreVersions: true,
		},
		wants: wants{
			rule:   "Host(`example.com`) && PathRegexp(`^/some/path/v2(_\\d+)?/foo(/|$)`)",
			prefix: "^/some/path/v2(_\\d+)?/foo"},
	}, {
		name: "v2, followed by other segment, for traefikV2",
		args: args{
			url:                     mustURLParse(t, "http://example.com/some/path/v2/foo"),
			matchUnderscoreVersions: true,
			traefikV2:               true,
		},
		wants: wants{
			rule:   "Host(`example.com`) && PathPrefix(`/{path:some/path/v2(_\\d+)?/foo(/|$)}`)",
			prefix: "^/some/path/v2(_\\d+)?/foo"},
	}, {
		name: "v1_3",
		args: args{
			url:                     mustURLParse(t, "http://example.com/some/path/v1_3"),
			matchUnderscoreVersions: true,
		},
		wants: wants{
			rule:   "Host(`example.com`) && PathRegexp(`^/some/path/v1(_3)?(/|$)`)",
			prefix: "^/some/path/v1(_3)?"},
	}, {
		name: "combined (never happens)",
		args: args{
			url:                     mustURLParse(t, "http://example.com/some/path/v345/v666_78"),
			matchUnderscoreVersions: true,
		},
		wants: wants{
			rule:   "Host(`example.com`) && PathRegexp(`^/some/path/v345(_\\d+)?/v666(_78)?(/|$)`)",
			prefix: "^/some/path/v345(_\\d+)?/v666(_78)?"},
	}, {
		name: "trailing slash in base url",
		args: args{
			url:                     mustURLParse(t, "http://example.com/some/path/v1/"),
			matchUnderscoreVersions: true,
		},
		wants: wants{
			rule:   "Host(`example.com`) && PathRegexp(`^/some/path/v1(_\\d+)?/`)",
			prefix: "^/some/path/v1(_\\d+)?/"},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, prefix := createIngressRuleAndStripPrefixForBaseURL(tt.args.url, tt.args.includelocalhost, tt.args.matchUnderscoreVersions, tt.args.traefikV2)
			if rule != tt.wants.rule {
				t.Errorf("createIngressRuleAndStripPrefixForBaseURL() = `%v`, _,\nwant rule `%v`", rule, tt.wants.rule)
			}
			if prefix != tt.wants.prefix {
				t.Errorf("createIngressRuleAndStripPrefixForBaseURL() = _, `%v`,\nwant prefix `%v`", prefix, tt.wants.prefix)
			}
		})
	}
}

func mustURLParse(t *testing.T, in string) url.URL {
	parsed, err := url.Parse(in)
	require.NoError(t, err)
	return *parsed
}
