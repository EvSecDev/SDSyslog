package fsops

import (
	"strings"
	"testing"
)

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		expectErr string
	}{
		{name: "simple relative path", path: "foo/bar.txt"},
		{name: "backslash separators", path: `foo\bar.txt`},
		{name: "dot segments and trailing separator", path: "./foo/"},
		{name: "dots inside a name", path: "a..b"},
		{name: "leading dots inside a name", path: "..foo"},

		{name: "empty path", path: "", expectErr: "path is empty"},
		{name: "NUL byte", path: "a\x00b", expectErr: "control character 0x00"},
		{name: "newline byte", path: "a\nb", expectErr: "control character 0x0a"},
		{name: "traversal segment", path: "..", expectErr: "traversal"},
		{name: "traversal in the middle", path: "a/../b", expectErr: "traversal"},
		{name: "traversal with mixed separators", path: `a\..\b`, expectErr: "traversal"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidatePath(test.path)
			if test.expectErr == "" {
				if err != nil {
					t.Fatalf("expected no error, but got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, but got nil", test.expectErr)
			}
			if !strings.Contains(err.Error(), test.expectErr) {
				t.Fatalf("expected error containing %q, but got: %v", test.expectErr, err)
			}
		})
	}
}
