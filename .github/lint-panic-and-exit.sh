#!/usr/bin/env bash
set -e

# Disallow usages of panic and os.Exit

SCRIPT_PATH=$( cd "$(dirname "${BASH_SOURCE[0]}")" ; pwd -P )
EXCLUDE_DIRECTORIES="--exclude-dir=examples --exclude-dir=.git --exclude-dir=.github "

if grep -R $EXCLUDE_DIRECTORIES -e 'os.Exit(' "$SCRIPT_PATH/.." | grep -v nolint; then
	echo "os.Exit( may only be used in example code"
	exit 1
fi

if grep -R $EXCLUDE_DIRECTORIES -e 'panic(' "$SCRIPT_PATH/.." | grep -v nolint; then
	echo "panic() may only be used in example code"
	exit 1
fi
