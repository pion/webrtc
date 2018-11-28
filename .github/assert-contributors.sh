#!/usr/bin/env bash
set -e

# Unshallow the repo, this check doesn't work with this enabled
# https://github.com/travis-ci/travis-ci/issues/3412
if [ -f $(git rev-parse --git-dir)/shallow ]; then
	git fetch --unshallow || true
fi

SCRIPT_PATH=$( cd "$(dirname "${BASH_SOURCE[0]}")" ; pwd -P )

CONTRIBUTORS=()
EXCLUDED_CONTIBUTORS=('John R. Bradley')
MISSING_CONTIBUTORS=()

shouldBeIncluded () {
	for i in "${EXCLUDED_CONTIBUTORS[@]}"
	do
		if [ "$i" == "$1" ] ; then
			return 1
		fi
	done
	return 0
}


IFS=$'\n' #Only split on newline
for contributor in $(git log --format='%aN' | sort -u)
do
	if shouldBeIncluded $contributor; then
		if ! grep -q "$contributor" "$SCRIPT_PATH/../README.md"; then
			MISSING_CONTIBUTORS+=("$contributor")
		fi
	fi
done
unset IFS

if [ ${#MISSING_CONTIBUTORS[@]} -ne 0 ]; then
    echo "Please add the following contributors to the README"
    for i in "${MISSING_CONTIBUTORS[@]}"
    do
	    echo "$i"
    done
    exit 1
fi
