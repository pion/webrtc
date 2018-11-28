#!/usr/bin/env bash
set -e

display_commit_message_error() {
cat << EndOfMessage
$1

-------------------------------------------------
The preceding commit message is invalid
it failed '$2' of the following checks

* Separate subject from body with a blank line
* Limit the subject line to 50 characters
* Capitalize the subject line
* Do not end the subject line with a period
* Wrap the body at 72 characters
EndOfMessage

     exit 1
}

lint_commit_message() {
    if [[ "$(echo "$1" | awk 'NR == 2 {print $1;}' | wc -c)" -ne 1 ]]; then
        display_commit_message_error "$1" 'Separate subject from body with a blank line'
    fi

    if [[ "$(echo "$1" | head -n1 | wc -m)" -gt 50 ]]; then
        display_commit_message_error "$1" 'Limit the subject line to 50 characters'
    fi

    if [[ ! $1 =~ ^[A-Z] ]]; then
        display_commit_message_error "$1" 'Capitalize the subject line'
    fi

    if [[ "$(echo "$1" | awk 'NR == 1 {print substr($0,length($0),1)}')" == "." ]]; then
        display_commit_message_error "$1" 'Do not end the subject line with a period'
    fi

    if [[ "$(echo "$1" | awk '{print length}' | sort -nr | head -1)" -gt 72 ]]; then
        display_commit_message_error "$1" 'Wrap the body at 72 characters'
    fi
}

if [ "$#" -eq 1 ]; then
   if [ ! -f "$1" ]; then
       echo "$0 was passed one argument, but was not a valid file"
       exit 1
   fi
   lint_commit_message "$(sed -n '/# Please enter the commit message for your changes. Lines starting/q;p' "$1")"
else
    # TRAVIS_COMMIT_RANGE is empty for initial branch commit
    if [[ "${TRAVIS_COMMIT_RANGE}" != *"..."* ]]; then
        parent=$(git log -n 1 --format="%P" ${TRAVIS_COMMIT_RANGE})
        TRAVIS_COMMIT_RANGE="${TRAVIS_COMMIT_RANGE}...$parent"
    fi

    for commit in $(git rev-list ${TRAVIS_COMMIT_RANGE}); do
      lint_commit_message "$(git log --format="%B" -n 1 $commit)"
    done
fi
