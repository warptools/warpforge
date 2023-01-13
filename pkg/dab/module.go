package dab

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/serum-errors/go-serum"

	"github.com/warptools/warpforge/wfapi"
)

const (
	MagicFilename_Module = "module.wf"
	MagicFilename_Plot   = "plot.wf"
)

// See validateDNS1123Subdomain and ValidateModuleName
const (
	// similar to dns1123 label hunks, but allows mid-string dots also.
	validation_moduleNamePathHunk_regexpStr string = "[a-z0-9]([-a-z0-9\\.]*[a-z0-9])?"
	validation_moduleNamePathHunk_msg       string = "must consist of lower case alphanumeric characters or '-' or '.', and must start and end with an alphanumeric character"
	validation_moduleNamePathHunk_maxlen    int    = 63
	validation_dns1123Label_regexpStr       string = "[a-z0-9]([-a-z0-9]*[a-z0-9])?"
	validation_dns1123Label_msg             string = "must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character"
	validation_dns1123Label_maxlen          int    = 63
	validation_dns1123Subdomain_regexpStr   string = validation_dns1123Label_regexpStr + "(\\." + validation_dns1123Label_regexpStr + ")*"
	validation_dns1123Subdomain_msg         string = "must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character"
	validation_dns1123Subdomain_maxlen      int    = 253
)

var (
	validation_moduleNamePathHunk_regexp = regexp.MustCompile("^" + validation_moduleNamePathHunk_regexpStr + "$")
	validation_dns1123Subdomain_regexp   = regexp.MustCompile("^" + validation_dns1123Subdomain_regexpStr + "$")
)

// ValidateModuleName checks the module name for invalid strings.
//
// Examples of valid module names:
//    - foobar
//    - foo.bar/grill
//    - foo-bar
//
// "Path segments" are defined as the segments separated by forward slash "/".
// "Domain segments" are defined as the segments separated by dot "." and only applies to the first path segment.
//
// A module name must resemble a domain name (per DNS RFC 1123 & 1035) with optional subsequent path segments. I.E.
//
//    [[[...]]]subdomain.]subdomain.]domain[/path[/morepath[...]]]
//
// The rules are summarized as following:
//    - Name MUST contain only:
//         - ASCII lowercase alpha-numeric characters
//         - hyphens '-'
//         - dots '.'
//         - forward slash '/'.
//    - Name MUST start AND end each path or domain segment with an ASCII lowercase alpha-numeric character.
//    - First path segment of the name MUST be 253 characters or less
//    - Each domain segment MUST only include ASCII, lowercase, alpha-numeric characters and hyphens '-'
//    - Each domain segment MUST be 63 characters or less
//
// Errors:
//
//  - warpforge-error-module-invalid -- when module name is invalid
func ValidateModuleName(moduleName wfapi.ModuleName) error {
	name := string(moduleName)
	parts := strings.Split(name, "/")
	if err := validateDNS1123Subdomain(parts[0]); err != nil {
		// test first segment separately so we can add a more specific error message
		return serum.Error(wfapi.ECodeModuleInvalid,
			serum.WithDetail("name", escape(name)),
			serum.WithCause(err),
		)
	}
	for _, pathHunk := range parts[1:] {
		if err := validateModuleNamePathHunk(pathHunk); err != nil {
			return serum.Error(wfapi.ECodeModuleInvalid,
				serum.WithDetail("name", escape(name)),
				serum.WithCause(err),
			)
		}
	}
	return nil
}

func validateModuleNamePathHunk(value string) error {
	if len(value) > validation_moduleNamePathHunk_maxlen {
		return serum.Error(wfapi.ECodeInvalid,
			serum.WithMessageTemplate("value must be no more than {{limit}} characters"),
			serum.WithDetail("limit", strconv.Itoa(validation_moduleNamePathHunk_maxlen)),
		)
	}
	if !validation_moduleNamePathHunk_regexp.MatchString(value) {
		return serum.Error(wfapi.ECodeInvalid,
			serum.WithMessageLiteral(validation_moduleNamePathHunk_msg),
		)
	}
	return nil
}

// escape limits strings to ascii characters. This will help when debugging
// unicode characters which are invisible, whitespace, or look-alikes to other characters.
// escape will convert any such characters into escape sequences.
func escape(name string) string {
	qName := strconv.QuoteToASCII(name)
	qName = qName[1 : len(qName)-1] // remove double quotes
	return qName
}

// validateDNS1123Subdomain implements restrictions for DNS names
//
// Based on of RFC 1123 (section 2.5)
//    - MUST allow host name to begin with a digit
//    - MUST allow host names of up to 63 characters
//    - SHOULD allow host names of up to 255 characters
// NOTE: We've shortened subdomain maximum length to 253 characters because
// that's what we did before and what kubernetes does.
//
// Based on RFC 1035 (secion 2.3)
// <<<
//     <domain> ::= <subdomain> | " "
//
//     <subdomain> ::= <label> | <subdomain> "." <label>
//
//     <label> ::= <letter> [ [ <ldh-str> ] <let-dig> ]
//
//     <ldh-str> ::= <let-dig-hyp> | <let-dig-hyp> <ldh-str>
//
//     <let-dig-hyp> ::= <let-dig> | "-"
//
//     <let-dig> ::= <letter> | <digit>
//
//     <letter> ::= any one of the 52 alphabetic characters A through Z in
//     upper case and a through z in lower case
//
//     <digit> ::= any one of the ten digits 0 through 9
//
//     Note that while upper and lower case letters are allowed in domain
//     names, no significance is attached to the case.  That is, two names with
//     the same spelling but different case are to be treated as if identical.
//
//     The labels must follow the rules for ARPANET host names.  They must
//     start with a letter, end with a letter or digit, and have as interior
//     characters only letters, digits, and hyphen.  There are also some
//     restrictions on the length.  Labels must be 63 characters or less.
// >>>
// We do not implement case folding and therefore domains MUST be lowercase.
//
// Errors:
//
//   - warpforge-error-invalid -- when the string fails validation
func validateDNS1123Subdomain(value string) error {
	if len(value) > validation_dns1123Subdomain_maxlen {
		return serum.Error(wfapi.ECodeInvalid,
			serum.WithMessageTemplate("domain must be no more than {{limit}} characters"),
			serum.WithDetail("limit", strconv.Itoa(validation_dns1123Subdomain_maxlen)),
		)
	}
	if !validation_dns1123Subdomain_regexp.MatchString(value) {
		return serum.Error(wfapi.ECodeInvalid,
			serum.WithMessageLiteral(validation_dns1123Subdomain_msg),
		)
	}
	lastDotIdx := 0
	for i, r := range value {
		if r == '.' {
			lastDotIdx = i + 1
		}
		if i-lastDotIdx >= validation_dns1123Label_maxlen {
			return serum.Error(wfapi.ECodeInvalid,
				serum.WithMessageTemplate("subdomain must be no more than {{limit}} characters"),
				serum.WithDetail("limit", strconv.Itoa(validation_dns1123Label_maxlen)),
				serum.WithDetail("label", value[lastDotIdx+1:i]),
			)
		}
	}
	return nil
}

// ModuleFromFile loads a wfapi.Module from filesystem path.
//
// In typical usage, the filename parameter will have the suffix of MagicFilename_Module.
//
// Errors:
//
// 	- warpforge-error-io -- for errors reading from fsys.
// 	- warpforge-error-serialization -- for errors from try to parse the data as a Module.
// 	- warpforge-error-datatoonew -- if encountering unknown data from a newer version of warpforge!
//  - warpforge-error-module-invalid -- when module name is invalid
func ModuleFromFile(fsys fs.FS, filename string) (wfapi.Module, error) {
	const situation = "loading a module"
	if filepath.IsAbs(filename) {
		filename = filename[1:]
	}
	f, err := fs.ReadFile(fsys, filename)
	if err != nil {
		return wfapi.Module{}, wfapi.ErrorIo(situation, filename, err)
	}

	moduleCapsule := wfapi.ModuleCapsule{}
	_, err = ipld.Unmarshal(f, json.Decode, &moduleCapsule, wfapi.TypeSystem.TypeByName("ModuleCapsule"))
	if err != nil {
		return wfapi.Module{}, wfapi.ErrorSerialization(situation, err)
	}
	if moduleCapsule.Module == nil {
		// ... this isn't really reachable.
		return wfapi.Module{}, wfapi.ErrorDataTooNew(situation, fmt.Errorf("no v1 Module in ModuleCapsule"))
	}

	if err := ValidateModuleName(moduleCapsule.Module.Name); err != nil {
		return wfapi.Module{}, err
	}

	return *moduleCapsule.Module, nil
}

// PlotFromFile loads a wfapi.Plot from filesystem path.
//
// In typical usage, the filename parameter will have the suffix of MagicFilename_Plot.
//
// Errors:
//
// 	- warpforge-error-io -- for errors reading from fsys.
// 	- warpforge-error-serialization -- for errors from try to parse the data as a Plot.
// 	- warpforge-error-datatoonew -- if encountering unknown data from a newer version of warpforge!
func PlotFromFile(fsys fs.FS, filename string) (wfapi.Plot, error) {
	const situation = "loading a plot"

	if filepath.IsAbs(filename) {
		filename = filename[1:]
	}
	f, err := fs.ReadFile(fsys, filename)
	if err != nil {
		return wfapi.Plot{}, wfapi.ErrorIo(situation, filename, err)
	}

	plotCapsule := wfapi.PlotCapsule{}
	_, err = ipld.Unmarshal(f, json.Decode, &plotCapsule, wfapi.TypeSystem.TypeByName("PlotCapsule"))
	if err != nil {
		return wfapi.Plot{}, wfapi.ErrorSerialization(situation, err)
	}
	if plotCapsule.Plot == nil {
		// ... this isn't really reachable.
		return wfapi.Plot{}, wfapi.ErrorDataTooNew(situation, fmt.Errorf("no v1 Plot in PlotCapsule"))
	}

	return *plotCapsule.Plot, nil
}

// FindModule looks for a module file on the filesystem and returns the first one found,
// searching directories upward.
//
// It searches from `join(basisPath,searchPath)` up to `basisPath`
// (in other words, it won't search above basisPath).
// Invoking it with an empty string for `basisPath` and cwd for `searchPath` is typical.
//
// If no module file is found, it will return nil for the error value.
// If errors are returned, they're due to filesystem IO.
//
// An fsys handle is required, but is typically `os.DirFS("/")` outside of tests.
//
// Errors:
//
//    - warpforge-error-searching-filesystem -- when an unexpected error occurs traversing the search path
func FindModule(fsys fs.FS, basisPath, searchPath string) (path string, remainingSearchPath string, err error) {
	// Our search loops over searchPath, popping a path segment off at the end of every round.
	//  Keep the given searchPath in hand; we might need it for an error report.
	basisPath = filepath.Clean(basisPath)
	searchAt := filepath.Join(basisPath, searchPath)
	for {
		path := filepath.Join(searchAt, MagicFilename_Module)
		_, err := fs.Stat(fsys, path)
		if err == nil {
			return path, filepath.Dir(searchAt), nil
		}
		if errors.Is(err, fs.ErrNotExist) { // no such thing?  oh well.  pop a segment and keep looking.
			// Went all the way up to basis path and didn't find it.
			// return NotFound
			if searchAt == basisPath {
				return "", "", nil
			}
			searchAt = filepath.Dir(searchAt)
			// ... otherwise: continue, with popped searchAt.
			continue
		}
		// You're still here?  That means there's an error, but of some unpleasant kind.
		//  Whatever this error is, our search has blind spots: error out.
		return "", searchAt, wfapi.ErrorSearchingFilesystem("module", err)
	}
}
