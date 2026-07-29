# Logical authentication schema

This document is database-neutral. The PostgreSQL draft is one possible physical representation and may be replaced after the database decision.

| Entity | Durable responsibility | Sensitive fields |
| --- | --- | --- |
| User | Stable Login subject, normalized email, status, verification time | Email |
| Password credential | Password verifier and change time | Argon2id encoded hash |
| Identity | External OIDC provider subject mapped to one Login user | Provider claims |
| Email challenge | Short-lived registration or verification attempt | HMAC digest only |
| Password reset | Single-use password reset authorization | HMAC digest only |
| Session | Browser session, expiry, revocation, device context | SHA-256 digest only |
| OIDC state | State, nonce and PKCE transaction state | Encrypted PKCE verifier |
| Audit event | Authentication security event without secrets | IP and user agent |

Required transactional invariants for the future database adapter:

- Consuming an email challenge and creating the corresponding user must be atomic.
- Consuming a password-reset token, updating the password, and revoking sessions must be atomic.
- A session, reset token, challenge, or OIDC state can be consumed or revoked at most once.
- Normalized email and `(provider, provider_subject)` are globally unique.
- Raw passwords, verification codes, reset tokens, session tokens, access tokens, and refresh tokens must never be persisted or written to logs.
