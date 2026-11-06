CREATE TABLE users (
 id uuid PRIMARY KEY,
 display_name varchar(100) NOT NULL CHECK (length(trim(display_name)) > 0),
 email varchar(254) NOT NULL CHECK (email = lower(email) AND length(email) > 3),
 password_hash text NOT NULL,
 role text NOT NULL CHECK (role IN ('CUSTOMER', 'HOST')),
 is_active boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_unique ON users (lower(email));
CREATE TABLE sessions (
 id uuid PRIMARY KEY,
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 token_hash text NOT NULL UNIQUE CHECK (token_hash ~ '^[0-9a-f]{64}$'),
 expires_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL,
 revoked_at timestamptz,
 CHECK (expires_at > created_at),
 CHECK (revoked_at IS NULL OR revoked_at >= created_at)
);
CREATE INDEX sessions_user_id ON sessions(user_id);
CREATE INDEX sessions_expires_at ON sessions(expires_at);
CREATE TABLE restaurants (
 id uuid PRIMARY KEY,
 name varchar(120) NOT NULL,
 timezone varchar(64) NOT NULL DEFAULT 'Asia/Bangkok'
);
CREATE TABLE service_queues (
 id uuid PRIMARY KEY,
 restaurant_id uuid NOT NULL REFERENCES restaurants(id) ON DELETE RESTRICT,
 name varchar(120) NOT NULL,
 ticket_prefix varchar(3) NOT NULL CHECK (ticket_prefix ~ '^[A-Z]{1,3}$'),
 is_active boolean NOT NULL DEFAULT true
);
CREATE TABLE operator_queue_permissions (
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 queue_id uuid NOT NULL REFERENCES service_queues(id) ON DELETE CASCADE,
 PRIMARY KEY(user_id,queue_id)
);
