CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    login         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    balance       NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (balance >= 0),
    withdrawn     NUMERIC(12,2) NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE orders (
    number      TEXT PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id),
    status      TEXT NOT NULL DEFAULT 'NEW' CHECK (status IN ('NEW', 'PROCESSING', 'INVALID', 'PROCESSED')),
    accrual     NUMERIC(12,2),
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_orders_user_id_uploaded_at ON orders (user_id, uploaded_at DESC);
CREATE INDEX idx_orders_status ON orders (status);

CREATE TABLE withdrawals (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT NOT NULL REFERENCES users(id),
    order_number  TEXT NOT NULL UNIQUE,
    sum           NUMERIC(12,2) NOT NULL CHECK (sum > 0),
    processed_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_withdrawals_user_id_processed_at ON withdrawals (user_id, processed_at DESC);
