Module Names
---

Best practices for module names it to make them look similar to a DNS host names and URL path.

```
[[[...]]]subdomain.]subdomain.]domain[/path[/morepath[...]]]
```

We place restrictions on module names which ensures that they would be valid as such, but not all names which would be valid domains/URL paths are considered valid module names.
As such our rules are heavily influenced by rules specified in [RFC-1123](https://datatracker.ietf.org/doc/html/rfc1123#section-2) and [RFC-1035](https://datatracker.ietf.org/doc/html/rfc1035#section-2.3.1).

Rules:
  - Must contain only:
    - ASCII lowercase alpha-numeric characters: `a-z0-9`.
    - hyphens: `-`.
    - dots: `.`.
    - forward slash: `/`.
  - Name MUST start AND end each path or domain segment with an ASCII lowercase alpha-numeric character.
  - First path segment of the name MUST be 253 characters or less
  - Each domain segment MUST only include ASCII, lowercase, alpha-numeric characters and hyphens '-'
  - Each domain segment MUST be 63 characters or less


# Valid Names

These names are valid.

[testmark]:# (valid/names)
```
my-domain.org
my-domain.org/path/to/thing
my.subdomain.domain.org/path-to-thing
my-domain.org/path.to.thing
```

These are _also_ valid but are not recommended. Prefer to use a DNS host name that you own as the domain for your modules.

[testmark]:# (valid/not-recommended)
```
my-domain
my-domain/path/to/thing
```


# Invalid Names

Names must not contain segments which are only dots. This would cause errors in filesystem projections.
Additionally this breaks DNS naming rules where a name must begin with an alpha-numeric character.

[testmark]:# (invalid/dots)
```
.
..
...
./foo
../foo
.../foo
my-domain.org/.
my-domain.org/..
my-domain.org/...
my-domain.org/./foo
my-domain.org/../foo
my-domain.org/.../foo
```

Names are not allowed to contain uppercase characters.

We do this even where it might be allowed because we don't implement case-folding.
Projecting case-insensitive systems onto case-sensitive filesystems can be problematic.

[testmark]:# (invalid/uppercase_domain)
```
mYdoMaIn
my-domain.org/myPath
```

Segments must begin and end with an alpha-numeric character. That means no hyphens as the last character.

[testmark]:# (invalid/hyphen/prefix)
```
  -my-domain.org
  my-domain.org/-foo
  my-domain.org/foo/-bar/grill
```

[testmark]:# (invalid/hyphen/suffix)
```
  my-domain.org-
  my-domain.org/foo-
  my-domain.org/foo/bar-/grill
```

Segments may not be empty. That means no trailing slashes.

[testmark]:# (invalid/trailing-slash)
```
  /my-domain.org
  my-domain.org/
  my-domain.org/foo/
  my-domain.org/foo/bar/
```


We don't allow underscores, but are strongly considering allowing them within **path** segments.
If you feel that underscores would be nice-to-have or useful for you then let us know!

[testmark]:# (invalid/underscores/in_path)
```
my-domain.org/a_path/to_a_thing
```

However we will NOT allow underscores as the _first_ character of any path. This would conflict with our
filesystem projections which use `_thing` for internal files.

[testmark]:# (invalid/underscores/prefix_always_invalid)
```
domain/_path
domain/path/_to/thing
```

Using underscores as a suffix will likely be invalid as long as hyphens are invalid suffixes.

[testmark]:# (invalid/underscores/suffix)
```
domain/path_
domain/path/to_/thing
```

Underscores within domain segments will remain invalid.

[testmark]:# (invalid/underscore/domain_always_invalid)
```
_domain
_domain.org
my_domain  
my_domain.org
```

Whitespace is not allowed in module names.

[testmark]:# (invalid/whitespace)
```
  my domain
  my domain.org
  mydomain.org/foo bar
```

Other punctuation is invalid in paths.

[testmark]:# (invalid/punctuation/paths)
```
my-domain.org/foo:bar
my-domain.org/foo!bar
my-domain.org/foo~bar
my-domain.org/foo;bar
my-domain.org/foo'bar
my-domain.org/foo"bar
my-domain.org/foo`bar
my-domain.org/foo#bar
my-domain.org/foo$bar
my-domain.org/foo%bar
my-domain.org/foo&bar
my-domain.org/foo(bar
my-domain.org/foo)bar
my-domain.org/foo*bar
my-domain.org/foo+bar
my-domain.org/foo,bar
my-domain.org/foo\bar
my-domain.org/foo—bar
my-domain.org/foo<bar
my-domain.org/foo>bar
my-domain.org/foo=bar
my-domain.org/foo?bar
my-domain.org/foo@bar
my-domain.org/foo[bar
my-domain.org/foo]bar
my-domain.org/foo^bar
my-domain.org/foo{bar
my-domain.org/foo}bar
my-domain.org/foo|bar
```

Other punctuation is invalid in domains.

[testmark]:# (invalid/punctuation/domains)
```
my:domain.org/foo/bar
my!domain.org/foo/bar
my~domain.org/foo/bar
my;domain.org/foo/bar
my'domain.org/foo/bar
my"domain.org/foo/bar
my`domain.org/foo/bar
my#domain.org/foo/bar
my$domain.org/foo/bar
my%domain.org/foo/bar
my&domain.org/foo/bar
my(domain.org/foo/bar
my)domain.org/foo/bar
my*domain.org/foo/bar
my+domain.org/foo/bar
my,domain.org/foo/bar
my\domain.org/foo/bar
my—domain.org/foo/bar
my<domain.org/foo/bar
my>domain.org/foo/bar
my=domain.org/foo/bar
my?domain.org/foo/bar
my@domain.org/foo/bar
my[domain.org/foo/bar
my]domain.org/foo/bar
my^domain.org/foo/bar
my{domain.org/foo/bar
my}domain.org/foo/bar
my|domain.org/foo/bar
```
