.PHONY: web-check api-check sqlc-check compose-check dev

web-check:
	npm test
	npm run lint
	npm run typecheck
	npm run build

api-check:
	cd apps/api && go test -race ./... && go vet ./... && go build -buildvcs=false -o /tmp/dinequeue-api ./cmd/api

sqlc-check:
	cd apps/api && sqlc diff

compose-check:
	docker compose config --quiet

dev:
	docker compose up --build
