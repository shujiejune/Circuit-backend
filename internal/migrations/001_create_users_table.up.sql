CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TYPE auth_provider AS ENUM ('EMAIL', 'GOOGLE');
CREATE TYPE machine_type AS ENUM ('DRONE', 'ROBOT');
CREATE TYPE machine_status AS ENUM ('IDLE', 'IN_TRANSIT', 'CHARGING', 'MAINTENANCE');
CREATE TYPE order_status AS ENUM ('PENDING_PAYMENT', 'CONFIRMED', 'IN_PROGRESS', 'DELIVERED', 'CANCELLED', 'FAILED');

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nickname VARCHAR(50) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    -- Nullable because users signing up with Google/OAuth won't have a password.
    password_hash VARCHAR(255),

    avatar_url VARCHAR(255),

    -- Columns for handling different authentication methods (e.g., email vs. Google)
    auth_provider auth_provider NOT NULL DEFAULT 'EMAIL',
    auth_provider_id VARCHAR(255), -- Stores the user's unique ID from the OAuth provider

    is_active BOOLEAN NOT NULL DEFAULT false,
    -- Columns for the email activation flow
    activation_token VARCHAR(255),
    activation_token_expires_at TIMESTAMPTZ,

    -- Columns for the "forgot password" flow
    password_reset_token VARCHAR(255),
    password_reset_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Ensures a user can only sign up once with a specific Google account
    CONSTRAINT unique_oauth_user UNIQUE (auth_provider, auth_provider_id)
);
-- Index on the users table for fast email lookups during login.
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
