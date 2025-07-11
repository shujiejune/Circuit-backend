-- This script seeds the database with initial data for development and testing.

-- Seed Users
INSERT INTO users (id, name, email, password_hash) VALUES
('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'Alice', 'alice@example.com', '$2a$10$hTyELZ8pkgk8xPNmvVz8WOFh5.b4RyjIqKHzJg1F3Xg1ZwHPDAJgO')
ON CONFLICT (email) DO NOTHING;

-- Seed Addresses for Alice
INSERT INTO addresses (id, user_id, label, street_address, is_default) VALUES
('b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a21', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'Home', '123 Main St, San Francisco, CA 94105', true),
('b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22', 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'Work', '456 Market St, San Francisco, CA 94104', false)
ON CONFLICT (id) DO NOTHING;

-- Seed Machines (one drone, one robot)
INSERT INTO machines (id, type, status, current_location, battery_level) VALUES
('d0eebc99-9c0b-4ef8-bb6d-6bb9bd380a31', 'DRONE', 'IDLE', ST_SetSRID(ST_MakePoint(-122.4000, 37.7880), 4326), 95),
('d0eebc99-9c0b-4ef8-bb6d-6bb9bd380a32', 'ROBOT', 'IDLE', ST_SetSRID(ST_MakePoint(-122.4194, 37.7749), 4326), 88)
ON CONFLICT (id) DO NOTHING;

-- Seed a completed order for Alice to populate history
INSERT INTO orders (id, user_id, machine_id, pickup_address_id, dropoff_address_id, status, item_description, item_weight_kg, cost)
SELECT
    'c0eebc99-9c0b-4ef8-bb6d-6bb9bd380a13',
    u.id,
    m.id,
    a_pickup.id,
    a_dropoff.id,
    'DELIVERED',
    'Fried Chicken',
    0.5,
    4.50
FROM
    users u
JOIN
    machines m ON m.type = 'DRONE'
JOIN
    addresses a_pickup ON u.id = a_pickup.user_id AND a_pickup.label = 'Work'
JOIN
    addresses a_dropoff ON u.id = a_dropoff.user_id AND a_dropoff.label = 'Home'
WHERE
    u.email = 'alice@example.com'
LIMIT 1
ON CONFLICT (id) DO NOTHING;
