#!/usr/bin/env bash
# CI entry: DineQueue release/build safety tests, no cloud or image build.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
python3 -m unittest discover -s "$ROOT/deploy/tests" -p 'test_*.py' -v
