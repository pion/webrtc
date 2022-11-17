#!/usr/bin/env bash

set -x
EXE="/assets/webexec"
CONF="/config/webexec"

/etc/init.d/ssh start
cp $EXE /usr/local/bin
rm -rf /home/runner/.local
mkdir -p /home/runner/.config/webexec
cp -r "$CONF" /home/runner/.config/
chown -R runner /home/runner
su -c "$EXE start --debug" runner
