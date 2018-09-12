#!/usr/bin/env bash
set -e

# Disallow usages of functions that cause the program to exit in the library code
SCRIPT_PATH=$( cd "$(dirname "${BASH_SOURCE[0]}")" ; pwd -P )
EXCLUDE_DIRECTORIES="--exclude-dir=examples --exclude-dir=.git --exclude-dir=.github "
DISALLOWED_FUNCTIONS=('os.Exit(' 'panic(' 'Fatal(' 'Fatalf(' 'Fatalln(')


for disallowedFunction in "${DISALLOWED_FUNCTIONS[@]}"
do
	if grep -R $EXCLUDE_DIRECTORIES -e "$disallowedFunction" "$SCRIPT_PATH/.." | grep -v -e '_test.go' -e 'nolint'; then
		echo "$disallowedFunction may only be used in example code"
		exit 1
	fi
done
