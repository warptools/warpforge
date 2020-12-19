using runc
==========

- all of the containment you wanted; none of the centralized and questionably-designed storage systems that you didn't.


unfortunate sharp edges
-----------------------

### initial process missing

The error handling for the command binary being any of:

- missing
- wrong permissions
- not executable in context (due to dynamic linking, wrong executable format, etc)

... is still atrocious:

> standard_init_linux.go:219: exec user process caused: no such file or directory

Part of this is to blame on the linux kernel itself being sketchily vague in its error codes in this vicinity.
But this is also not a very pleasing string to have to pattern match on if you want to handle it.

This can be compensated for mostly by looking for the problems ahead of time.
Technically that's a TOCTOU violation, but around containers, "it's fine", c'est la vie.
