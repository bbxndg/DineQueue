# SQL migrations

No domain migrations have been implemented in the repository foundation.

Add reviewed, versioned PostgreSQL SQL migrations before implementing the S1 domain
schema. GORM persistence models must follow that schema; application startup must not
call AutoMigrate. Keep foreign keys, CHECK constraints, partial unique indexes and
data-preserving schema changes explicit. Migration tooling/version is still to be selected
with the first schema contract; no migration runner is implied by this directory.
