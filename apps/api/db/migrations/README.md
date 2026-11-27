# SQL migrations

`0001_auth.sql` creates F-01 users, hashed sessions, restaurant/service queue and
operator grants. `0002_queue_entries.sql` (F-02) adds daily ticket counters, queue
entries and their CHECK/partial unique indexes. `0003_customer_readiness.sql` adds Lab 7 readiness. `0004_floor_inventory.sql` adds the floor and MANUAL policy defaults. `0005_seating_lifecycle.sql` adds group assignments/memberships, action records/events and additive terminal lifecycle fields. Existing queues remain MANUAL.

Add reviewed, versioned PostgreSQL SQL migrations when extending the S1 domain
schema. GORM persistence models must follow that schema; application startup must not
call AutoMigrate. Keep foreign keys, CHECK constraints, partial unique indexes and
data-preserving schema changes explicit.

From `apps/api`, `go run -buildvcs=false ./cmd/db migrate` applies embedded migrations
under an advisory lock and one transaction. Applied checksums are immutable; add a
new migration rather than editing an applied file. `go run -buildvcs=false ./cmd/db seed`
creates demo accounts from environment (disabled in production). Neither runs on
API startup. Existing accounts/passwords and revoked grants are preserved by seed.
Do not run seed against a live customer database. Back up before schema changes;
no destructive down command is provided.
