ALTER TABLE service_queues
 ADD COLUMN operations_mode text NOT NULL DEFAULT 'MANUAL' CHECK (operations_mode IN ('MANUAL','SEATING')),
 ADD COLUMN hold_ttl_seconds integer NOT NULL DEFAULT 300 CHECK (hold_ttl_seconds BETWEEN 60 AND 1800);

CREATE TABLE restaurant_floor_state (
 restaurant_id uuid PRIMARY KEY REFERENCES restaurants(id) ON DELETE RESTRICT,
 layout_version bigint NOT NULL DEFAULT 1 CHECK (layout_version > 0)
);
CREATE TABLE floor_zones (
 id uuid PRIMARY KEY,
 restaurant_id uuid NOT NULL REFERENCES restaurants(id) ON DELETE RESTRICT,
 name varchar(100) NOT NULL CHECK (length(trim(name)) > 0),
 is_active boolean NOT NULL DEFAULT true,
 UNIQUE (restaurant_id,id),
 UNIQUE (restaurant_id,name)
);
CREATE TABLE dining_tables (
 id uuid PRIMARY KEY,
 restaurant_id uuid NOT NULL,
 zone_id uuid NOT NULL,
 code varchar(20) NOT NULL CHECK (length(trim(code)) > 0),
 capacity integer NOT NULL CHECK (capacity BETWEEN 1 AND 8),
 status text NOT NULL DEFAULT 'AVAILABLE' CHECK (status IN ('AVAILABLE','HELD','OCCUPIED','DIRTY','CLEANING','OUT_OF_SERVICE')),
 position_x double precision NOT NULL CHECK (position_x >= 0 AND position_x <= 100),
 position_y double precision NOT NULL CHECK (position_y >= 0 AND position_y <= 100),
 version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
 updated_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY (restaurant_id,zone_id) REFERENCES floor_zones(restaurant_id,id) ON DELETE RESTRICT,
 UNIQUE (restaurant_id,code),
 UNIQUE (restaurant_id,zone_id,id)
);
CREATE TABLE table_connections (
 restaurant_id uuid NOT NULL,
 zone_id uuid NOT NULL,
 table_low_id uuid NOT NULL,
 table_high_id uuid NOT NULL,
 is_active boolean NOT NULL DEFAULT true,
 approved_by uuid REFERENCES users(id) ON DELETE RESTRICT,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY (table_low_id,table_high_id),
 CHECK (table_low_id < table_high_id),
 FOREIGN KEY (restaurant_id,zone_id,table_low_id) REFERENCES dining_tables(restaurant_id,zone_id,id) ON DELETE RESTRICT,
 FOREIGN KEY (restaurant_id,zone_id,table_high_id) REFERENCES dining_tables(restaurant_id,zone_id,id) ON DELETE RESTRICT
);
CREATE INDEX dining_tables_zone ON dining_tables(restaurant_id,zone_id);
INSERT INTO restaurant_floor_state(restaurant_id) SELECT id FROM restaurants;
