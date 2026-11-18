#!/usr/bin/env bash
# DineQueue EP5: safe by default. See deploy/README.md before --apply.
set -euo pipefail
cd "$(dirname "$0")/.."
exec python3 deploy/first_deploy.py "$@"
