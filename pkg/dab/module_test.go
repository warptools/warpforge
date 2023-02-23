package dab_test

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	qt "github.com/frankban/quicktest"
	"github.com/serum-errors/go-serum"
	"github.com/warpfork/go-testmark"

	"github.com/warptools/warpforge/pkg/dab"
	"github.com/warptools/warpforge/wfapi"
)

var veryLongDomain = strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 61)
var manySubdomains = strings.Repeat("a.", 126) + "b"

func TestValidateModuleName_Testmark(t *testing.T) {
	filename := "../../examples/200-module-parse/module-names.md"
	t.Logf("file://%s", filename)
	doc, err := testmark.ReadFile(filename)
	qt.Assert(t, err, qt.IsNil)

	for _, hunk := range doc.DataHunks {
		hunk := hunk
		t.Run(hunk.Name, func(t *testing.T) {
			lines := strings.Split(string(hunk.Body), "\n")
			for idx, line := range lines {
				if line == "" {
					continue
				}
				line := line
				tname := fmt.Sprintf(":%d/%s", hunk.LineStart+3+idx, line)
				t.Run(tname, func(t *testing.T) {
					err := dab.ValidateModuleName(wfapi.ModuleName(line))
					if strings.HasPrefix(hunk.Name, "valid/") {
						qt.Assert(t, err, qt.IsNil)
						return
					}
					qt.Assert(t, err, qt.IsNotNil)
				})
			}
		})
	}
}

// These tests should expand on checks in the testmark tests
func TestValidateModuleName(t *testing.T) {
	qt.Assert(t, veryLongDomain, qt.HasLen, 253)
	qt.Assert(t, manySubdomains, qt.HasLen, 253)
	for _, testcase := range []struct {
		name    string // if left empty will use the value name
		value   string
		checker qt.Checker
	}{
		// good names | happy path
		{"", "example", qt.IsNil},
		{"", "example.com", qt.IsNil},
		{"", "example.com/b", qt.IsNil},
		{"", "a.b/c", qt.IsNil},
		{"", "a.b.c", qt.IsNil},
		{"", "abc.def.ghi/jkl", qt.IsNil},
		{"", "abc-def.ghi/jkl", qt.IsNil},
		{"", "example.com/b/c", qt.IsNil},
		{"", "1.2", qt.IsNil},
		{"", "1.2/3", qt.IsNil},
		{"", "1.2/3/4", qt.IsNil},
		{"", "a.b/foo", qt.IsNil},
		{"", "a.b/foobar", qt.IsNil},
		{"", "a.b/foo/bar", qt.IsNil},
		// underscore considered for valid chars but removed
		{"", "abc_def.ghi/jkl", qt.IsNotNil},
		{"", "a.b/foo_bar", qt.IsNotNil},
		// cannot start or end segment with non-alphanumeric characters
		{"", ".a.b/foo/bar", qt.IsNotNil},
		{"", "_a.b/foo/bar", qt.IsNotNil},
		{"", "-a.b/foo/bar", qt.IsNotNil},
		{"", "a.b./foo/bar", qt.IsNotNil},
		{"", "a.b_/foo/bar", qt.IsNotNil},
		{"", "a.b-/foo/bar", qt.IsNotNil},
		{"", "a.b/.foo/bar", qt.IsNotNil},
		{"", "a.b/_foo/bar", qt.IsNotNil},
		{"", "a.b/-foo/bar", qt.IsNotNil},
		{"", "a.b/foo./bar", qt.IsNotNil},
		{"", "a.b/foo_/bar", qt.IsNotNil},
		{"", "a.b/foo-/bar", qt.IsNotNil},
		{"", "a.b/foo/.bar", qt.IsNotNil},
		{"", "a.b/foo/_bar", qt.IsNotNil},
		{"", "a.b/foo/-bar", qt.IsNotNil},
		{"", "a.b/foo/bar.", qt.IsNotNil},
		{"", "a.b/foo/bar_", qt.IsNotNil},
		{"", "a.b/foo/bar-", qt.IsNotNil},
		// punctuation
		{"", "a.b/foo:bar", qt.IsNotNil},
		{"", "a.b/foo!bar", qt.IsNotNil},
		{"", "a.b/foo~bar", qt.IsNotNil},
		{"", "a.b/foo;bar", qt.IsNotNil},
		{"", "a.b/foo'bar", qt.IsNotNil},
		{"", `a.b/foo"bar`, qt.IsNotNil},
		{"", "a.b/foo`bar", qt.IsNotNil},
		{"", "a.b/foo#bar", qt.IsNotNil},
		{"", "a.b/foo$bar", qt.IsNotNil},
		{"", "a.b/foo%bar", qt.IsNotNil},
		{"", "a.b/foo&bar", qt.IsNotNil},
		{"", "a.b/foo(bar", qt.IsNotNil},
		{"", "a.b/foo)bar", qt.IsNotNil},
		{"", "a.b/foo*bar", qt.IsNotNil},
		{"", "a.b/foo+bar", qt.IsNotNil},
		{"", "a.b/foo,bar", qt.IsNotNil},
		{"", `a.b/foo\bar`, qt.IsNotNil},
		{"", "a.b/foo—bar", qt.IsNotNil},
		{"", "a.b/foo<bar", qt.IsNotNil},
		{"", "a.b/foo>bar", qt.IsNotNil},
		{"", "a.b/foo=bar", qt.IsNotNil},
		{"", "a.b/foo?bar", qt.IsNotNil},
		{"", "a.b/foo@bar", qt.IsNotNil},
		{"", "a.b/foo[bar", qt.IsNotNil},
		{"", "a.b/foo]bar", qt.IsNotNil},
		{"", "a.b/foo^bar", qt.IsNotNil},
		{"", "a.b/foo{bar", qt.IsNotNil},
		{"", "a.b/foo}bar", qt.IsNotNil},
		{"", "a.b/foo|bar", qt.IsNotNil},
		// segments containing only dots
		{"", ".", qt.IsNotNil},
		{"", "..", qt.IsNotNil},
		{"", "...", qt.IsNotNil},
		{"", "./a", qt.IsNotNil},
		{"", "../a", qt.IsNotNil},
		{"", ".../a", qt.IsNotNil},
		{"", "a.b/.", qt.IsNotNil},
		{"", "a.b/..", qt.IsNotNil},
		{"", "a.b/...", qt.IsNotNil},
		{"", "a.b/./b", qt.IsNotNil},
		{"", "a.b/../b", qt.IsNotNil},
		{"", "a.b/.../b", qt.IsNotNil},
		// segments beginning with underscores
		{"", "_a.b", qt.IsNotNil},
		{"", "a.b/_foo", qt.IsNotNil},
		{"", "_a.b/foo/bar", qt.IsNotNil},
		{"", "a.b/_foo/bar", qt.IsNotNil},
		{"", "a.b/foo/_bar", qt.IsNotNil},
		// empty segments
		{"", "", qt.IsNotNil},
		{"", "a.b/", qt.IsNotNil},
		{"", "/foo", qt.IsNotNil},
		{"", "a.b//foo", qt.IsNotNil},
		{"", "a.b/foo//bar", qt.IsNotNil},
		// whitespace
		{"", " ", qt.IsNotNil},
		{"", "a.b/foo ", qt.IsNotNil}, // ends with space
		{"", "a.b/foo\t", qt.IsNotNil},
		{"", "a.b/foo\n", qt.IsNotNil},
		{"", "a.b/foo\r", qt.IsNotNil},
		{"", "a.b/foo\v", qt.IsNotNil},
		{"", "a.b/foo\f", qt.IsNotNil},
		{"", "a.b/foo\u00A0", qt.IsNotNil}, //NBSP
		{"", "a.b/foo\u0085", qt.IsNotNil}, //NEL
		{"", "a.b/foo bar", qt.IsNotNil},
		{"", "a.b/foo\tbar", qt.IsNotNil},
		{"", "a.b/foo\nbar", qt.IsNotNil},
		{"", "a.b/foo\rbar", qt.IsNotNil},
		{"", "a.b/foo\vbar", qt.IsNotNil},
		{"", "a.b/foo\fbar", qt.IsNotNil},
		{"", "a.b/foo\u00A0bar", qt.IsNotNil}, //NBSP
		{"", "a.b/foo\u0085bar", qt.IsNotNil}, //NEL
		{"", "a.b/foo/bar ", qt.IsNotNil},     // ends with space
		// control codes
		{"", "\b", qt.IsNotNil},
		{"", "a.b/\b", qt.IsNotNil},
		{"", "a.b/foo\b", qt.IsNotNil},
		{"", "a.b/foo/bar\b", qt.IsNotNil},
		{"", "a.b/foo\bbar", qt.IsNotNil},
		// uppercase
		{"", "A", qt.IsNotNil},
		{"", "A.b", qt.IsNotNil},
		{"", "a.B", qt.IsNotNil},
		{"", "A.b.c", qt.IsNotNil},
		{"", "a.B.c", qt.IsNotNil},
		{"", "a.b.C", qt.IsNotNil},
		{"", "a.b.c/D", qt.IsNotNil},
		{"", "a.b.c/D/e", qt.IsNotNil},
		{"", "a.b.c/d/E", qt.IsNotNil},
		// length
		{"64 chars", strings.Repeat("a", 64), qt.IsNotNil}, // DNS label limit
		{"63 chars", strings.Repeat("a", 63), qt.IsNil},
		{"63*2 chars", strings.Repeat("a", 63) + "." + strings.Repeat("b", 63), qt.IsNil},
		{fmt.Sprintf("%d chars", len(veryLongDomain)), veryLongDomain, qt.IsNil},
		{fmt.Sprintf("%d chars", len(veryLongDomain)+1), veryLongDomain + "x", qt.IsNotNil},
		{"many subdomains", manySubdomains, qt.IsNil},
		{"one too many subdomains", manySubdomains + ".x", qt.IsNotNil},
		{"pathlen 63", "example.org/" + strings.Repeat("a", 63), qt.IsNil},
		{"pathlen 64", "example.org/" + strings.Repeat("a", 64), qt.IsNotNil},
	} {
		testcase := testcase
		name := testcase.name
		if name == "" {
			name = fmt.Sprintf("%#v", testcase.value)
		}
		t.Run(name, func(t *testing.T) {
			t.Logf("%#v", testcase.value)
			moduleName := wfapi.ModuleName(testcase.value)
			result := dab.ValidateModuleName(moduleName)
			qt.Assert(t, result, testcase.checker)
		})
	}
}

func TestFindModule(t *testing.T) {
	modulePath := filepath.Join("foo/bar", dab.MagicFilename_Module)
	defaultFS := fstest.MapFS{
		modulePath: &fstest.MapFile{},
	}
	toplevelModuleFs := fstest.MapFS{
		dab.MagicFilename_Module: &fstest.MapFile{},
	}

	for _, tt := range []struct {
		name   string
		fsys   fs.FS
		basis  string
		search string
		expect string
		remain string
		err    error
	}{
		{
			name:   "top-level module; empty paths",
			fsys:   toplevelModuleFs,
			basis:  "",
			search: "",
			expect: dab.MagicFilename_Module,
		},
		{
			name:   "top-level module: subdir search relative",
			fsys:   toplevelModuleFs,
			basis:  "",
			search: "sub/dir/path",
			expect: dab.MagicFilename_Module,
		},
		{
			name:   "top-level module: subdir search absolute",
			fsys:   toplevelModuleFs,
			basis:  "",
			search: "/sub/dir/path",
			expect: dab.MagicFilename_Module,
		},
		{
			name:   "top-level module: absolute basis ",
			fsys:   toplevelModuleFs,
			basis:  "/",
			search: "sub/dir/path",
			expect: dab.MagicFilename_Module,
		},
		{
			name:   "top-level module: absolute basis and absolute search path ",
			fsys:   toplevelModuleFs,
			basis:  "/",
			search: "sub/dir/path",
			expect: dab.MagicFilename_Module,
		},
		{
			name:   "top-level module: unreachable due to basis",
			fsys:   toplevelModuleFs,
			basis:  "/sub/dir",
			search: "",
			expect: "", // not found
			remain: "sub",
		},
		{
			name:   "empty paths",
			basis:  "",
			search: "",
			expect: "", // not found
		},
		{
			name:   "relative exact search path",
			basis:  "",
			search: "foo/bar",
			expect: modulePath,
			remain: "foo",
		},
		{
			name:   "relative subdir search path",
			basis:  "",
			search: "foo/bar/non/existing/dir",
			expect: modulePath,
			remain: "foo",
		},
		{
			name:   "relative incorrect search path",
			basis:  "",
			search: "non/existing/dir",
			expect: "",
			remain: "",
		},
		{
			name:   "absolute exact search path",
			basis:  "",
			search: "/foo/bar",
			expect: modulePath,
			remain: "foo",
		},
		{
			name:   "absolute subdir search path",
			basis:  "",
			search: "/foo/bar/non/existing/dir",
			expect: modulePath,
			remain: "foo",
		},
		{
			name:   "absolute incorrect search path",
			basis:  "",
			search: "/non/existing/dir",
			expect: "", // not found
			remain: "",
			err:    nil,
		},
		{
			name:   "absolute basis path, relative exact search path",
			basis:  "/foo",
			search: "bar",
			expect: modulePath,
			remain: "foo",
		},
		{
			name:   "absolute basis path, relative subdir search path",
			basis:  "/foo",
			search: "bar/non/existing/dir",
			expect: modulePath,
			remain: "foo",
		},
		{
			name:   "absolute basis path, incorrect relative search path",
			basis:  "/foo",
			search: "non/existing/dir",
			expect: "", // not found
			remain: "",
		},
		{
			name:   "absolute exact basis path, empty search path",
			basis:  "/foo/bar",
			search: "",
			expect: modulePath,
			remain: "foo",
		},
		{
			name:   "absolute exact basis path, relative subdir search path",
			basis:  "/foo/bar",
			search: "non/existing/dir",
			expect: modulePath,
			remain: "foo",
		},
		{
			name:   "relative exact basis path, relative subdir search path",
			basis:  "foo/bar",
			search: "non/existing/dir",
			expect: modulePath,
			remain: "foo",
		},
		{
			name:   "unreachable relative exact basis path, relative subdir search path",
			basis:  "foo/bar/grill",
			search: "non/existing/dir",
			expect: "",
			remain: "foo/bar",
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if tt.fsys == nil {
				tt.fsys = defaultFS
			}
			result, remain, err := dab.FindModule(tt.fsys, tt.basis, tt.search)
			if tt.err != nil {
				qt.Assert(t, err, qt.IsNotNil)
				qt.Assert(t, err.Error(), qt.Equals, tt.err.Error())
				qt.Assert(t, result, qt.Equals, tt.expect)
				qt.Assert(t, remain, qt.Equals, tt.remain)
				return
			}
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, result, qt.Equals, tt.expect)
			qt.Assert(t, remain, qt.Equals, tt.remain)
		})
	}
}

func TestSubPathRel(t *testing.T) {
	for _, tt := range []struct {
		name   string
		base   string
		targ   string
		expect string
		err    error
	}{
		{name: "relative base",
			base: "",
			targ: "/",
			err:  serum.Errorf(wfapi.ECodeInvalid, "base path %q must be absolute", ""),
		},
		{name: "relative target",
			base: "/",
			targ: "",
			err:  serum.Errorf(wfapi.ECodeInvalid, "target path %q must be absolute", ""),
		},
		{name: "target not subdirectory of base",
			base: "/bar",
			targ: "/foo",
			err:  serum.Errorf(wfapi.ECodeInvalid, "target path %q is not relative to basepath %q", "/foo", "/bar"),
		},
		{name: "both same root dir",
			base:   "/",
			targ:   "/",
			expect: "",
		},
		{name: "both same single segment",
			base:   "/foo",
			targ:   "/foo",
			expect: "",
		},
		{name: "both same double segment",
			base:   "/foo/bar",
			targ:   "/foo/bar",
			expect: "",
		},
		{name: "depth 1 from root",
			base:   "/",
			targ:   "/foo",
			expect: "foo",
		},
		{name: "depth greater than 1 from root",
			base:   "/",
			targ:   "/foo/bar/grill",
			expect: "foo/bar/grill",
		},
		{name: "depth 1 from path",
			base:   "/foo/bar",
			targ:   "/foo/bar/grill",
			expect: "grill",
		},
		{name: "depth greater than 1 from path",
			base:   "/foo/bar",
			targ:   "/foo/bar/grill/baz",
			expect: "grill/baz",
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			result, err := dab.SubPathRel(tt.base, tt.targ)
			if tt.err != nil {
				qt.Assert(t, err, qt.IsNotNil)
				qt.Assert(t, result, qt.Equals, "")
				qt.Assert(t, err.Error(), qt.Equals, tt.err.Error())
				return
			}
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, result, qt.Equals, tt.expect)
		})
	}
}
