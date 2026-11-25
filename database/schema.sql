-- Database: db_bioskop

CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    full_name VARCHAR(100) NOT NULL,
    email VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    phone VARCHAR(20),
    role VARCHAR(20) NOT NULL DEFAULT 'CUSTOMER' CHECK (role IN ('ADMIN', 'CUSTOMER')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS cities (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    province VARCHAR(100) NOT NULL
);

CREATE TABLE IF NOT EXISTS cinemas (
    id BIGSERIAL PRIMARY KEY,
    city_id BIGINT NOT NULL REFERENCES cities(id) ON DELETE RESTRICT,
    name VARCHAR(150) NOT NULL,
    address TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS studios (
    id BIGSERIAL PRIMARY KEY,
    cinema_id BIGINT NOT NULL REFERENCES cinemas(id) ON DELETE CASCADE,
    name VARCHAR(50) NOT NULL,
    studio_type VARCHAR(30) NOT NULL DEFAULT 'REGULAR',
    total_seats INT NOT NULL CHECK (total_seats > 0),
    UNIQUE (cinema_id, name)
);

CREATE TABLE IF NOT EXISTS seats (
    id BIGSERIAL PRIMARY KEY,
    studio_id BIGINT NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    row_label VARCHAR(5) NOT NULL,
    seat_number INT NOT NULL,
    seat_type VARCHAR(20) NOT NULL DEFAULT 'REGULAR',
    UNIQUE (studio_id, row_label, seat_number)
);

CREATE TABLE IF NOT EXISTS movies (
    id BIGSERIAL PRIMARY KEY,
    title VARCHAR(200) NOT NULL,
    genre VARCHAR(100),
    duration_minutes INT NOT NULL CHECK (duration_minutes > 0),
    rating VARCHAR(10),
    synopsis TEXT,
    release_date DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS schedules (
    id BIGSERIAL PRIMARY KEY,
    movie_id BIGINT NOT NULL REFERENCES movies(id) ON DELETE RESTRICT,
    studio_id BIGINT NOT NULL REFERENCES studios(id) ON DELETE RESTRICT,
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    price NUMERIC(12, 2) NOT NULL CHECK (price >= 0),
    status VARCHAR(20) NOT NULL DEFAULT 'SCHEDULED' CHECK (status IN ('SCHEDULED', 'ONGOING', 'COMPLETED', 'CANCELLED')),
    cancellation_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (end_time > start_time)
);

CREATE TABLE IF NOT EXISTS bookings (
    id BIGSERIAL PRIMARY KEY,
    booking_code VARCHAR(30) NOT NULL UNIQUE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    schedule_id BIGINT NOT NULL REFERENCES schedules(id) ON DELETE RESTRICT,
    total_amount NUMERIC(12, 2) NOT NULL CHECK (total_amount >= 0),
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'PAID', 'EXPIRED', 'CANCELLED', 'REFUNDED')),
    expired_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS booking_seats (
    id BIGSERIAL PRIMARY KEY,
    booking_id BIGINT NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
    schedule_id BIGINT NOT NULL REFERENCES schedules(id) ON DELETE RESTRICT,
    seat_id BIGINT NOT NULL REFERENCES seats(id) ON DELETE RESTRICT,
    price NUMERIC(12, 2) NOT NULL CHECK (price >= 0),
    status VARCHAR(20) NOT NULL DEFAULT 'LOCKED' CHECK (status IN ('LOCKED', 'BOOKED', 'RELEASED', 'REFUNDED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_active_schedule_seat
    ON booking_seats (schedule_id, seat_id)
    WHERE status IN ('LOCKED', 'BOOKED');

CREATE TABLE IF NOT EXISTS payments (
    id BIGSERIAL PRIMARY KEY,
    booking_id BIGINT NOT NULL UNIQUE REFERENCES bookings(id) ON DELETE RESTRICT,
    payment_method VARCHAR(50) NOT NULL,
    gateway_reference VARCHAR(100) UNIQUE,
    amount NUMERIC(12, 2) NOT NULL CHECK (amount >= 0),
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'SUCCESS', 'FAILED', 'EXPIRED')),
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS refunds (
    id BIGSERIAL PRIMARY KEY,
    booking_id BIGINT NOT NULL UNIQUE REFERENCES bookings(id) ON DELETE RESTRICT,
    payment_id BIGINT NOT NULL REFERENCES payments(id) ON DELETE RESTRICT,
    idempotency_key VARCHAR(100) NOT NULL UNIQUE,
    refund_amount NUMERIC(12, 2) NOT NULL CHECK (refund_amount >= 0),
    reason TEXT NOT NULL,
    initiated_by VARCHAR(20) NOT NULL DEFAULT 'CINEMA' CHECK (initiated_by IN ('CINEMA', 'CUSTOMER')),
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'PROCESSING', 'SUCCESS', 'FAILED')),
    refunded_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_schedules_studio_time ON schedules (studio_id, start_time, end_time);
CREATE INDEX IF NOT EXISTS idx_schedules_movie_status ON schedules (movie_id, status);
CREATE INDEX IF NOT EXISTS idx_bookings_status_expired ON bookings (status, expired_at);

-- Akun awal untuk pengujian:
-- 1. ADMIN    -> email: admin@cinema.com | password: admin123
-- 2. CUSTOMER -> email: arfa@cinema.com, zaim@cinema.com, al@cinema.com | password: password
INSERT INTO users (full_name, email, password_hash, phone, role) VALUES
    ('Admin Bioskop', 'admin@cinema.com', '$2a$10$l1UwuwWpBoBXhuTnWerf5ekMIjNR0AvAbPhfjyGz5sGdu821UTXfy', '081200000001', 'ADMIN'),
    ('Arfa', 'arfa@cinema.com', '$2a$10$uPL3LihY9pJGtYwaG25TquLHJZUm7.ZNMon33W1oL.XEfRYSeFaem', '081200000002', 'CUSTOMER'),
    ('Zaim', 'zaim@cinema.com', '$2a$10$uPL3LihY9pJGtYwaG25TquLHJZUm7.ZNMon33W1oL.XEfRYSeFaem', '081200000003', 'CUSTOMER'),
    ('Al', 'al@cinema.com', '$2a$10$uPL3LihY9pJGtYwaG25TquLHJZUm7.ZNMon33W1oL.XEfRYSeFaem', '081200000004', 'CUSTOMER')
ON CONFLICT (email) DO NOTHING;

INSERT INTO cities (name, province) VALUES
    ('Semarang', 'Jawa Tengah'),
    ('Surakarta', 'Jawa Tengah'),
    ('Jakarta Selatan', 'DKI Jakarta')
ON CONFLICT (name) DO NOTHING;

INSERT INTO cinemas (city_id, name, address) VALUES
    (1, 'Cinema Paragon Semarang', 'Jl. Pemuda No.118, Semarang'),
    (2, 'Cinema Solo Paragon', 'Jl. Yosodipuro No.133, Surakarta')
ON CONFLICT DO NOTHING;

INSERT INTO studios (cinema_id, name, studio_type, total_seats) VALUES
    (1, 'Studio 1', 'REGULAR', 100),
    (1, 'Studio 2', 'IMAX', 120),
    (2, 'Studio 1', 'REGULAR', 80)
ON CONFLICT (cinema_id, name) DO NOTHING;

INSERT INTO seats (studio_id, row_label, seat_number, seat_type)
SELECT s.studio_id, r.row_label, n.seat_number, 'REGULAR'
FROM (VALUES (1), (2), (3)) AS s(studio_id)
CROSS JOIN (VALUES ('A'), ('B'), ('C'), ('D'), ('E'), ('F')) AS r(row_label)
CROSS JOIN generate_series(1, 10) AS n(seat_number)
ON CONFLICT (studio_id, row_label, seat_number) DO NOTHING;

INSERT INTO movies (title, genre, duration_minutes, rating, synopsis, release_date) VALUES
    ('Dori The Explorer', 'Petualangan', 115, 'SU', 'Petualangan Dori menjelajahi pulau terpencil bersama teman-temannya.', '2026-10-12'),
    ('Ari n The Gank', 'Komedi', 105, 'R13+', 'Kisah persahabatan Ari dan gengnya menghadapi kekacauan sehari-hari.', '2026-11-01'),
    ('Badut Bopong', 'Horor/Komedi', 110, 'D17+', 'Teror kocak sekaligus mencekam dari badut keliling kampung.', '2026-11-20')
ON CONFLICT DO NOTHING;

INSERT INTO schedules (movie_id, studio_id, start_time, end_time, price, status) VALUES
    (1, 1, '2026-12-01 14:00:00+07', '2026-12-01 16:00:00+07', 45000.00, 'SCHEDULED'),
    (2, 2, '2026-12-01 19:00:00+07', '2026-12-01 20:50:00+07', 55000.00, 'SCHEDULED'),
    (3, 3, '2026-12-01 20:00:00+07', '2026-12-01 21:55:00+07', 50000.00, 'SCHEDULED')
ON CONFLICT DO NOTHING;
