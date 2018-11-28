#!/bin/bash

SCRIPT_PATH=$( cd "$(dirname "${BASH_SOURCE[0]}")" ; pwd -P )

cp "$SCRIPT_PATH/hooks/commit-msg.sh" "$SCRIPT_PATH/../.git/hooks/commit-msg"
cp "$SCRIPT_PATH/hooks/pre-commit.sh" "$SCRIPT_PATH/../.git/hooks/pre-commit"
cp "$SCRIPT_PATH/hooks/pre-push.sh" "$SCRIPT_PATH/../.git/hooks/pre-push"
