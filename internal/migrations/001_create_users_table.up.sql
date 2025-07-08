CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TYPE machine_type AS ENUM ('DRONE', 'ROBOT');
CREATE TYPE machine_status AS ENUM ('IDLE', 'IN_TRANSIT', 'CHARGING', 'MAINTENANCE');
CREATE TYPE order_status AS ENUM ('PENDING_PAYMENT', 'CONFIRMED', 'IN_PROGRESS', 'DELIVERED', 'CANCELLED', 'FAILED');

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    -- role VARCHAR(50) NOT NULL DEFAULT 'USER', -- ('USER', 'ADMIN')
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Index on the users table for fast email lookups during login.
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
