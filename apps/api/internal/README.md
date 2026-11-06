# API boundaries

One modular-monolith process. HTTP parsing/response concerns live in transport/http;
services enforce use-case rules, and GORM persistence adapters execute parameterized queries.

The foundation contains only the health service and GORM PostgreSQL connection/pool. Auth/queue
modules are not implemented. Add their handler/service/repository code under their
own modules when those contracts are authorized. A service must pass one transaction
handle (`tx *gorm.DB`) through every repository call in an atomic business operation; repositories
must never commit parts of a use case independently.

Persistence models must be separate from HTTP DTOs. Use explicit conditional updates
and check affected rows for state transitions; do not use Save as a transition shortcut.
Avoid arbitrary client-supplied SQL fragments or implicit association writes.

`db/migrations` is reserved for explicit versioned SQL migrations. No startup AutoMigrate,
sqlc generation or direct pgx pool is used. pgx remains an indirect driver dependency.
Domain models/migrations, seed accounts and authentication remain later S1 work.
