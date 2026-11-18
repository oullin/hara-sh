#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
make --no-print-directory -s -C "$root" "$root/bin/hara"
exec "$root/bin/hara" t3 backup "$@"
