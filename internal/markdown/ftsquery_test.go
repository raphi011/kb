package markdown

import "testing"

func TestConvertQuery(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"foo", `"foo"`},
		{"foo bar", `"foo" "bar"`},
		{`"foo bar"`, `"foo bar"`},
		{"foo*", `"foo"*`},
		{"-foo", `NOT "foo"`},
		{"foo|bar", `"foo" OR "bar"`},
		{"foo AND bar", `"foo" AND "bar"`},
		{"title:foo", `title:"foo"`},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ConvertQuery(tt.input)
			if got != tt.want {
				t.Errorf("ConvertQuery(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
