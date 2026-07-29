INSERT INTO auth_users (
    id, email, normalized_email, status, email_verified_at, created_at, updated_at
)
VALUES (
    '00000000-0000-4000-8000-000000000001',
    :'default_user_email',
    LOWER(:'default_user_email'),
    'active',
    NOW(),
    NOW(),
    NOW()
)
ON CONFLICT (id) DO UPDATE SET
    email = EXCLUDED.email,
    normalized_email = EXCLUDED.normalized_email,
    status = 'active',
    email_verified_at = COALESCE(auth_users.email_verified_at, NOW()),
    updated_at = NOW();

INSERT INTO auth_password_credentials (
    user_id, password_hash, password_changed_at, updated_at
)
VALUES (
    '00000000-0000-4000-8000-000000000001',
    :'default_user_password_hash',
    NOW(),
    NOW()
)
ON CONFLICT (user_id) DO UPDATE SET
    password_hash = EXCLUDED.password_hash,
    password_changed_at = NOW(),
    failed_attempts = 0,
    locked_until = NULL,
    updated_at = NOW();
