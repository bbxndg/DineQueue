#!/usr/bin/env bash
# DineQueue EP5 offline tests: fake GCP responses; no gcloud invocation.
set -euo pipefail
cd "$(dirname "$0")/../.."
python3 -m unittest discover -s deploy/tests -p 'test_first_deploy.py' -v
