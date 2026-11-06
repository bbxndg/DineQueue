# API boundaries

One modular-monolith process. HTTP parsing/response concerns live in transport/http;
services enforce use-case rules, and GORM persistence adapters execute parameterized queries.

The foundation contains health and GORM wiring. F-01 adds `internal/auth` for
password/session service, repository and HTTP guards, and `cmd/db` for opt-in SQL
migration/demo seed. F-02 adds `internal/queue` (active queue list and customer join
with daily ticket allocation); Host operations and ticket reads remain later S1 work. A service must pass one transaction
handle (`tx *gorm.DB`) through every repository call in an atomic business operation; repositories
must never commit parts of a use case independently.

Persistence models must be separate from HTTP DTOs. Use explicit conditional updates
and check affected rows for state transitions; do not use Save as a transition shortcut.
Avoid arbitrary client-supplied SQL fragments or implicit association writes.

`db/migrations` is reserved for explicit versioned SQL migrations. No startup AutoMigrate,
sqlc generation or direct pgx pool is used. pgx remains an indirect driver dependency.
Future queue routes
must use Authenticate plus role/queue/owner checks, and CSRF on unsafe requests.
Ownership comparisons must use server-loaded owner IDs. No test-only guard route is
registered by the production app.
