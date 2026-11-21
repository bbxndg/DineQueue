#!/usr/bin/env bash
# EP7: build tracked source, never the WIF credential workspace.
set -euo pipefail
cd "$(dirname "$0")/.."
: "${PROJECT_ID:?Required}" "${REGION:?Required}" "${IMAGE_TAG:?Required}"
[[ "$PROJECT_ID" =~ ^[a-z][a-z0-9-]{4,28}[a-z0-9]$ ]] || exit 1
[[ "$REGION" =~ ^[a-z]+-[a-z]+[0-9]$ ]] || exit 1
[[ "$IMAGE_TAG" =~ ^[a-f0-9]{40}$ ]] || exit 1
[[ "$(git rev-parse HEAD)" == "$IMAGE_TAG" ]] || { echo 'Build SHA must match checked-out source.' >&2; exit 1; }
python3 - <<'PY'
from pathlib import PurePosixPath
import subprocess
for path in subprocess.check_output(['git','ls-files','-z']).decode().split('\0'):
    if any(part in ('.aws','.npmrc') or part.startswith('gha-creds-') or
           (part.startswith('.env') and part != '.env.example')
           for part in PurePosixPath(path).parts):
        raise SystemExit('Tracked private configuration/credential file cannot enter a release build.')
PY
SOURCE_DIR="$(mktemp -d "${RUNNER_TEMP:-/tmp}/dinequeue-release-source.XXXXXX")"
trap 'rm -rf "$SOURCE_DIR"' EXIT
git archive --format=tar "$IMAGE_TAG" | tar -xf - -C "$SOURCE_DIR"
gcloud --project "$PROJECT_ID" --quiet auth configure-docker "$REGION-docker.pkg.dev"
REGISTRY="$REGION-docker.pkg.dev/$PROJECT_ID/dinequeue"
docker build --file "$SOURCE_DIR/apps/api/Dockerfile" --target production \
  --tag "$REGISTRY/api:$IMAGE_TAG" "$SOURCE_DIR"
docker build --file "$SOURCE_DIR/apps/api/Dockerfile" --target db-tools \
  --tag "$REGISTRY/db-tools:$IMAGE_TAG" "$SOURCE_DIR"
docker build --file "$SOURCE_DIR/apps/web/Dockerfile" --target production \
  --build-arg API_INTERNAL_URL=http://127.0.0.1:8081 \
  --tag "$REGISTRY/web:$IMAGE_TAG" "$SOURCE_DIR"
# Build all targets first. Any failed push stops the workflow before release.
for image in api web db-tools; do
  docker push "$REGISTRY/$image:$IMAGE_TAG"
done
