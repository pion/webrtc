#!/bin/bash -eu

# SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
# SPDX-License-Identifier: MIT

docker compose up -d

function on_exit {
  docker compose logs
  docker compose rm -fsv
}

trap on_exit EXIT

TIMEOUT=10
timeout $TIMEOUT docker compose logs -f | grep -q "answer  | Message from DataChannel"
timeout $TIMEOUT docker compose logs -f | grep -q "offer   | Message from DataChannel"
