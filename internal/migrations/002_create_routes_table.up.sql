CREATE TABLE routes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    polyline TEXT NOT NULL,
    distance_meters INTEGER,
    duration_seconds INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);