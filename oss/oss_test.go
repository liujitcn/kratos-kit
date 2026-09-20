package oss

import "testing"

// TestNormalizeObjectRootDirectory 验证对象存储根目录的兼容格式。
func TestNormalizeObjectRootDirectory(t *testing.T) {
	tests := []struct {
		name string
		root string
		want string
	}{
		{name: "dot relative", root: "./data", want: "data"},
		{name: "relative", root: "data/", want: "data"},
		{name: "absolute object prefix", root: "/data/", want: "data"},
		{name: "empty", root: "", want: ""},
		{name: "current directory", root: ".", want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := normalizeObjectRootDirectory(test.root); got != test.want {
				t.Fatalf("normalizeObjectRootDirectory(%q) = %q, want %q", test.root, got, test.want)
			}
		})
	}
}
