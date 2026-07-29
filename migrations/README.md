# Database migrations

`postgres/001_init.sql` creates the Login schema. `postgres/002_default_user.sql`
creates or updates the bootstrap administrator using psql variables supplied from
a Kubernetes Secret. Neither migration is executed automatically by the service.

Do not run it until these decisions are frozen:

1. Production database engine and minimum version.
2. Managed-database encryption, backup, restore, and retention policy.
3. Migration runner and rollback procedure.
4. Application database role and least-privilege grants.
5. OIDC PKCE verifier encryption key management.

Production uses the PostgreSQL `auth.Store`; local development may omit
`VF_DATABASE_URL` to use the in-memory implementation.
