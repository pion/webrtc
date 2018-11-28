#!/bin/sh

# Redirect output to stderr.
exec 1>&2

.github/lint-disallowed-functions-in-library.sh
