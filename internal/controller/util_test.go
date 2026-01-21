package controller

import (
	"net/url"
	"testing"

	"github.com/pdok/smooth-operator/model"
	"github.com/stretchr/testify/require"
)

func Test_createIngressRuleAndStripPrefixForURL(t *testing.T) {
	type args struct {
		url                     url.URL
		ingressRouteUrls        model.IngressRouteURLs
		includelocalhost        bool
		matchUnderscoreVersions bool
	}
	type wants struct {
		rule     string
		prefixes []string
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
			rule:     "Host(`example.com`)",
			prefixes: []string{},
		},
	}, {
		name: "include localhost",
		args: args{
			url:              mustURLParse(t, "http://example.com"),
			includelocalhost: true,
		},
		wants: wants{
			rule:     "(Host(`localhost`) || Host(`example.com`))",
			prefixes: []string{}},
	}, {
		name: "no versions",
		args: args{
			url: mustURLParse(t, "http://example.com/some/path"),
		},
		wants: wants{
			rule:     "Host(`example.com`) && PathRegexp(`^/some/path(/|$)`)",
			prefixes: []string{"^/some/path"}},
	}, {
		name: "v1, no replacing",
		args: args{
			url:                     mustURLParse(t, "http://example.com/some/path/v1"),
			matchUnderscoreVersions: false,
		},
		wants: wants{
			rule:     "Host(`example.com`) && PathRegexp(`^/some/path/v1(/|$)`)",
			prefixes: []string{"^/some/path/v1"}},
	}, {
		name: "v1, no replacing, trailing slash",
		args: args{
			url:                     mustURLParse(t, "http://example.com/some/path/v1/"),
			matchUnderscoreVersions: false,
		},
		wants: wants{
			rule:     "Host(`example.com`) && PathRegexp(`^/some/path/v1/`)",
			prefixes: []string{"^/some/path/v1/"}},
	}, {
		name: "v1",
		args: args{
			url:                     mustURLParse(t, "http://example.com/some/path/v1"),
			matchUnderscoreVersions: true,
		},
		wants: wants{
			rule:     "Host(`example.com`) && PathRegexp(`^/some/path/v1(_\\d+)?(/|$)`)",
			prefixes: []string{"^/some/path/v1(_\\d+)?"}},
	}, {
		name: "v2, followed by other segment",
		args: args{
			url:                     mustURLParse(t, "http://example.com/some/path/v2/foo"),
			matchUnderscoreVersions: true,
		},
		wants: wants{
			rule:     "Host(`example.com`) && PathRegexp(`^/some/path/v2(_\\d+)?/foo(/|$)`)",
			prefixes: []string{"^/some/path/v2(_\\d+)?/foo"}},
	}, {
		name: "v1_3",
		args: args{
			url:                     mustURLParse(t, "http://example.com/some/path/v1_3"),
			matchUnderscoreVersions: true,
		},
		wants: wants{
			rule:     "Host(`example.com`) && PathRegexp(`^/some/path/v1(_3)?(/|$)`)",
			prefixes: []string{"^/some/path/v1(_3)?"}},
	}, {
		name: "combined (never happens)",
		args: args{
			url:                     mustURLParse(t, "http://example.com/some/path/v345/v666_78"),
			matchUnderscoreVersions: true,
		},
		wants: wants{
			rule:     "Host(`example.com`) && PathRegexp(`^/some/path/v345(_\\d+)?/v666(_78)?(/|$)`)",
			prefixes: []string{"^/some/path/v345(_\\d+)?/v666(_78)?"}},
	}, {
		name: "trailing slash in base url",
		args: args{
			url:                     mustURLParse(t, "http://example.com/some/path/v1/"),
			matchUnderscoreVersions: true,
		},
		wants: wants{
			rule:     "Host(`example.com`) && PathRegexp(`^/some/path/v1(_\\d+)?/`)",
			prefixes: []string{"^/some/path/v1(_\\d+)?/"}},
	},
		{
			name: "more ingress routes",
			args: args{
				url: mustURLParse(t, "http://example.com/one/pathname/v1/"),
				ingressRouteUrls: model.IngressRouteURLs{{
					URL: model.URL{URL: asPtr(mustURLParse(t, "http://example.com/one/pathname/v1/"))},
				}, {
					URL: model.URL{URL: asPtr(mustURLParse(t, "http://example.com/two/otherpathname/v1/"))},
				}},
				matchUnderscoreVersions: true,
			},
			wants: wants{
				rule:     "Host(`example.com`) && PathRegexp(`^/one/pathname/v1(_\\d+)?/`)",
				prefixes: []string{"^/one/pathname/v1(_\\d+)?/", "^/two/otherpathname/v1(_\\d+)?/"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := getMatchRuleForURL(tt.args.url, tt.args.includelocalhost, tt.args.matchUnderscoreVersions)
			prefixes := getStripPrefixesRegexps(tt.args.url, tt.args.ingressRouteUrls, tt.args.matchUnderscoreVersions)

			if rule != tt.wants.rule {
				t.Errorf("createIngressRuleAndStripPrefixForBaseURL() = `%v`, _,\nwant rule `%v`", rule, tt.wants.rule)
			}

			if len(prefixes) != len(tt.wants.prefixes) {
				t.Errorf("getStripPrefixesRegexps() = number of prefixes `%v`,\nwant `%v`", len(prefixes), len(tt.wants.prefixes))
			} else {
				for i, _ := range prefixes {
					actualPrefix := prefixes[i]
					wantPrefix := tt.wants.prefixes[i]
					if actualPrefix != wantPrefix {
						t.Errorf("getStripPrefixesRegexps() = `%v`,\nwant prefix `%v`", actualPrefix, wantPrefix)
					}
				}
			}
		})
	}
}

func asPtr(myURL url.URL) *url.URL {
	return &myURL
}

func mustURLParse(t *testing.T, in string) url.URL {
	t.Helper()
	parsed, err := url.Parse(in)
	require.NoError(t, err)
	return *parsed
}
