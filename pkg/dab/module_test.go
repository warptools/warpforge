package dab_test

import (
	"fmt"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/warptools/warpforge/pkg/dab"
	"github.com/warptools/warpforge/wfapi"
)

func TestValidateModuleName(t *testing.T) {
	for _, testcase := range []struct {
		name    string
		checker qt.Checker
	}{
		// good names | happy path
		{"example.com", qt.IsNil},
		{"example.com/b", qt.IsNil},
		{"a.b/c", qt.IsNil},
		{"a.b.c", qt.IsNil},
		{"abc.def.ghi/jkl", qt.IsNil},
		{"abc_def.ghi/jkl", qt.IsNil},
		{"abc-def.ghi/jkl", qt.IsNil},
		{"example.com/b/c", qt.IsNil},
		{"1.2", qt.IsNil},
		{"1.2/3", qt.IsNil},
		{"1.2/3/4", qt.IsNil},
		{"a.b/foo", qt.IsNil},
		{"a.b/foobar", qt.IsNil},
		{"a.b/foo_bar", qt.IsNil},
		{"a.b/foo/bar", qt.IsNil},
		// cannot start or end segment with non-alphanumeric characters
		{".a.b/foo/bar", qt.IsNotNil},
		{"_a.b/foo/bar", qt.IsNotNil},
		{"-a.b/foo/bar", qt.IsNotNil},
		{"a.b./foo/bar", qt.IsNotNil},
		{"a.b_/foo/bar", qt.IsNotNil},
		{"a.b-/foo/bar", qt.IsNotNil},
		{"a.b/.foo/bar", qt.IsNotNil},
		{"a.b/_foo/bar", qt.IsNotNil},
		{"a.b/-foo/bar", qt.IsNotNil},
		{"a.b/foo./bar", qt.IsNotNil},
		{"a.b/foo_/bar", qt.IsNotNil},
		{"a.b/foo-/bar", qt.IsNotNil},
		{"a.b/foo/.bar", qt.IsNotNil},
		{"a.b/foo/_bar", qt.IsNotNil},
		{"a.b/foo/-bar", qt.IsNotNil},
		{"a.b/foo/bar.", qt.IsNotNil},
		{"a.b/foo/bar_", qt.IsNotNil},
		{"a.b/foo/bar-", qt.IsNotNil},
		// punctuation
		{"a.b/foo:bar", qt.IsNotNil},
		{"a.b/foo!bar", qt.IsNotNil},
		{"a.b/foo~bar", qt.IsNotNil},
		{"a.b/foo;bar", qt.IsNotNil},
		{"a.b/foo'bar", qt.IsNotNil},
		{`a.b/foo"bar`, qt.IsNotNil},
		{"a.b/foo`bar", qt.IsNotNil},
		{"a.b/foo#bar", qt.IsNotNil},
		{"a.b/foo$bar", qt.IsNotNil},
		{"a.b/foo%bar", qt.IsNotNil},
		{"a.b/foo&bar", qt.IsNotNil},
		{"a.b/foo(bar", qt.IsNotNil},
		{"a.b/foo)bar", qt.IsNotNil},
		{"a.b/foo*bar", qt.IsNotNil},
		{"a.b/foo+bar", qt.IsNotNil},
		{"a.b/foo,bar", qt.IsNotNil},
		{`a.b/foo\bar`, qt.IsNotNil},
		{"a.b/foo—bar", qt.IsNotNil},
		{"a.b/foo<bar", qt.IsNotNil},
		{"a.b/foo>bar", qt.IsNotNil},
		{"a.b/foo=bar", qt.IsNotNil},
		{"a.b/foo?bar", qt.IsNotNil},
		{"a.b/foo@bar", qt.IsNotNil},
		{"a.b/foo[bar", qt.IsNotNil},
		{"a.b/foo]bar", qt.IsNotNil},
		{"a.b/foo^bar", qt.IsNotNil},
		{"a.b/foo{bar", qt.IsNotNil},
		{"a.b/foo}bar", qt.IsNotNil},
		{"a.b/foo|bar", qt.IsNotNil},
		// segments containing only dots
		{".", qt.IsNotNil},
		{"..", qt.IsNotNil},
		{"...", qt.IsNotNil},
		{"./a", qt.IsNotNil},
		{"../a", qt.IsNotNil},
		{".../a", qt.IsNotNil},
		{"a.b/.", qt.IsNotNil},
		{"a.b/..", qt.IsNotNil},
		{"a.b/...", qt.IsNotNil},
		{"a.b/./b", qt.IsNotNil},
		{"a.b/../b", qt.IsNotNil},
		{"a.b/.../b", qt.IsNotNil},
		// segments beginning with underscores
		{"_a.b", qt.IsNotNil},
		{"a.b/_foo", qt.IsNotNil},
		{"_a.b/foo/bar", qt.IsNotNil},
		{"a.b/_foo/bar", qt.IsNotNil},
		{"a.b/foo/_bar", qt.IsNotNil},
		// empty segments
		{"", qt.IsNotNil},
		{"a.b/", qt.IsNotNil},
		{"/foo", qt.IsNotNil},
		{"a.b//foo", qt.IsNotNil},
		{"a.b/foo//bar", qt.IsNotNil},
		// whitespace
		{" ", qt.IsNotNil},
		{"a.b/foo ", qt.IsNotNil}, // ends with space
		{"a.b/foo\t", qt.IsNotNil},
		{"a.b/foo\n", qt.IsNotNil},
		{"a.b/foo\r", qt.IsNotNil},
		{"a.b/foo\v", qt.IsNotNil},
		{"a.b/foo\f", qt.IsNotNil},
		{"a.b/foo\u00A0", qt.IsNotNil}, //NBSP
		{"a.b/foo\u0085", qt.IsNotNil}, //NEL
		{"a.b/foo bar", qt.IsNotNil},
		{"a.b/foo\tbar", qt.IsNotNil},
		{"a.b/foo\nbar", qt.IsNotNil},
		{"a.b/foo\rbar", qt.IsNotNil},
		{"a.b/foo\vbar", qt.IsNotNil},
		{"a.b/foo\fbar", qt.IsNotNil},
		{"a.b/foo\u00A0bar", qt.IsNotNil}, //NBSP
		{"a.b/foo\u0085bar", qt.IsNotNil}, //NEL
		{"a.b/foo/bar ", qt.IsNotNil},     // ends with space
		// control codes
		{"\b", qt.IsNotNil},
		{"a.b/\b", qt.IsNotNil},
		{"a.b/foo\b", qt.IsNotNil},
		{"a.b/foo/bar\b", qt.IsNotNil},
		{"a.b/foo\bbar", qt.IsNotNil},
		// length
		{strings.Repeat("a", 62) + ".b", qt.IsNotNil},                              // DNS rules are 63 characters for domain names, we're loosely going to encourage compatibility by setting a length limit
		{strings.Repeat("a", 61) + ".b" + "/" + strings.Repeat("c", 64), qt.IsNil}, // only first segment is length limited
	} {
		testcase := testcase
		name := fmt.Sprintf("%#v", testcase.name)
		t.Run(name, func(t *testing.T) {
			t.Logf("%#v", testcase.name)
			moduleName := wfapi.ModuleName(testcase.name)
			result := dab.ValidateModuleName(moduleName)
			qt.Assert(t, result, testcase.checker)
		})
	}
}
