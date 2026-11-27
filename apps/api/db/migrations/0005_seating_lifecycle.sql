-- Additive seating lifecycle. Existing manual entries and credentials are retained.
ALTER TABLE queue_entries
 DROP CONSTRAINT queue_entries_status_check,
 DROP CONSTRAINT chk_queue_entry_lifecycle,
 ADD COLUMN cancelled_at timestamptz,
 ADD COLUMN cancelled_by uuid REFERENCES users(id),
 ADD COLUMN no_show_at timestamptz,
 ADD COLUMN no_show_by uuid REFERENCES users(id),
 ADD COLUMN no_show_reason text CHECK (no_show_reason IN ('CUSTOMER_ABSENT','HOLD_EXPIRED')),
 ADD CONSTRAINT queue_entries_status_check CHECK (status IN ('WAITING','CALLED','COMPLETED','CANCELLED','NO_SHOW')),
 ADD CONSTRAINT chk_queue_entry_lifecycle CHECK (
 (status='WAITING' AND called_at IS NULL AND called_by IS NULL AND completed_at IS NULL AND completed_by IS NULL AND cancelled_at IS NULL AND cancelled_by IS NULL AND no_show_at IS NULL AND no_show_by IS NULL AND no_show_reason IS NULL)
 OR (status='CALLED' AND called_at IS NOT NULL AND called_by IS NOT NULL AND completed_at IS NULL AND completed_by IS NULL AND cancelled_at IS NULL AND cancelled_by IS NULL AND no_show_at IS NULL AND no_show_by IS NULL AND no_show_reason IS NULL)
 OR (status='COMPLETED' AND called_at IS NOT NULL AND called_by IS NOT NULL AND completed_at IS NOT NULL AND completed_by IS NOT NULL AND cancelled_at IS NULL AND cancelled_by IS NULL AND no_show_at IS NULL AND no_show_by IS NULL AND no_show_reason IS NULL)
 OR (status='CANCELLED' AND called_at IS NULL AND called_by IS NULL AND completed_at IS NULL AND completed_by IS NULL AND cancelled_at IS NOT NULL AND cancelled_by=customer_id AND cancelled_by IS NOT NULL AND no_show_at IS NULL AND no_show_by IS NULL AND no_show_reason IS NULL)
 OR (status='NO_SHOW' AND called_at IS NOT NULL AND called_by IS NOT NULL AND completed_at IS NULL AND completed_by IS NULL AND cancelled_at IS NULL AND cancelled_by IS NULL AND no_show_at IS NOT NULL AND no_show_reason IS NOT NULL AND ((no_show_reason='HOLD_EXPIRED' AND no_show_by IS NULL) OR (no_show_reason='CUSTOMER_ABSENT' AND no_show_by IS NOT NULL)))) ,
 ADD CHECK (cancelled_at IS NULL OR cancelled_at>=joined_at),
 ADD CHECK (no_show_at IS NULL OR no_show_at>=called_at),
 ADD UNIQUE (queue_id,id);
ALTER TABLE service_queues ADD UNIQUE (restaurant_id,id);
CREATE TABLE seating_assignments (
 id uuid PRIMARY KEY,
 queue_id uuid NOT NULL,
 entry_id uuid NOT NULL,
 restaurant_id uuid NOT NULL,
 zone_id uuid NOT NULL,
 status text NOT NULL CHECK(status IN ('HELD','ACTIVE','CLOSED','RELEASED')),
 turnover_phase text CHECK(turnover_phase IN ('DIRTY','CLEANING','CLEANED')),
 version bigint NOT NULL DEFAULT 1 CHECK(version>0),
 capacity_snapshot integer NOT NULL CHECK(capacity_snapshot BETWEEN 1 AND 24),
 held_at timestamptz NOT NULL,
 hold_expires_at timestamptz NOT NULL CHECK(hold_expires_at>held_at),
 seated_at timestamptz,closed_at timestamptz,cleaning_started_at timestamptz,cleaned_at timestamptz,released_at timestamptz,
 release_reason text CHECK(release_reason IN ('HOLD_EXPIRED','CUSTOMER_ABSENT','CLEANED')),
 FOREIGN KEY(restaurant_id,queue_id) REFERENCES service_queues(restaurant_id,id),
 FOREIGN KEY(queue_id,entry_id) REFERENCES queue_entries(queue_id,id),
 FOREIGN KEY(restaurant_id,zone_id) REFERENCES floor_zones(restaurant_id,id),
 UNIQUE(restaurant_id,zone_id,id),
 CHECK (
 (status='HELD' AND seated_at IS NULL AND closed_at IS NULL AND turnover_phase IS NULL AND released_at IS NULL AND release_reason IS NULL)
 OR (status='ACTIVE' AND seated_at IS NOT NULL AND closed_at IS NULL AND turnover_phase IS NULL AND released_at IS NULL AND release_reason IS NULL)
 OR (status='CLOSED' AND seated_at IS NOT NULL AND closed_at IS NOT NULL AND turnover_phase IS NOT NULL AND ((turnover_phase IN ('DIRTY','CLEANING') AND released_at IS NULL AND release_reason IS NULL) OR (turnover_phase='CLEANED' AND released_at IS NOT NULL AND release_reason='CLEANED')))
 OR (status='RELEASED' AND seated_at IS NULL AND closed_at IS NULL AND turnover_phase IS NULL AND released_at IS NOT NULL AND release_reason IS NOT NULL AND release_reason IN ('HOLD_EXPIRED','CUSTOMER_ABSENT'))),
 CHECK(seated_at IS NULL OR (seated_at>=held_at AND seated_at<hold_expires_at)),
 CHECK(closed_at IS NULL OR closed_at>=seated_at),
 CHECK(released_at IS NULL OR released_at>=held_at),
 CHECK ((turnover_phase IS NULL AND cleaning_started_at IS NULL AND cleaned_at IS NULL) OR (turnover_phase='DIRTY' AND cleaning_started_at IS NULL AND cleaned_at IS NULL) OR (turnover_phase='CLEANING' AND cleaning_started_at>=closed_at AND cleaning_started_at IS NOT NULL AND cleaned_at IS NULL) OR (turnover_phase='CLEANED' AND cleaning_started_at IS NOT NULL AND cleaned_at IS NOT NULL AND cleaned_at>=cleaning_started_at))
);
CREATE UNIQUE INDEX seating_one_outstanding_entry ON seating_assignments(entry_id) WHERE status IN ('HELD','ACTIVE');
CREATE TABLE seating_assignment_tables (
 assignment_id uuid NOT NULL,
 table_id uuid NOT NULL,
 restaurant_id uuid NOT NULL,
 zone_id uuid NOT NULL,
 capacity_snapshot integer NOT NULL CHECK(capacity_snapshot BETWEEN 1 AND 8),
 released_at timestamptz,
 PRIMARY KEY(assignment_id,table_id),
 FOREIGN KEY(restaurant_id,zone_id,assignment_id) REFERENCES seating_assignments(restaurant_id,zone_id,id),
 FOREIGN KEY(restaurant_id,zone_id,table_id) REFERENCES dining_tables(restaurant_id,zone_id,id)
);
CREATE UNIQUE INDEX seating_one_live_table_claim ON seating_assignment_tables(table_id) WHERE released_at IS NULL;
CREATE TABLE operation_events (
 id uuid PRIMARY KEY,
 queue_id uuid NOT NULL REFERENCES service_queues(id),
 entry_id uuid NOT NULL,
 assignment_id uuid NOT NULL REFERENCES seating_assignments(id),
 actor_id uuid REFERENCES users(id),
 action text NOT NULL,
 from_state text NOT NULL,
 to_state text NOT NULL,
 metadata jsonb NOT NULL DEFAULT '{}',
 occurred_at timestamptz NOT NULL,
 FOREIGN KEY(queue_id,entry_id) REFERENCES queue_entries(queue_id,id)
);
CREATE TABLE operation_requests (
 actor_id uuid NOT NULL REFERENCES users(id),
 queue_id uuid NOT NULL REFERENCES service_queues(id),
 operation text NOT NULL,
 idempotency_key uuid NOT NULL,
 request_hash text NOT NULL,
 response jsonb NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY(actor_id,queue_id,operation,idempotency_key)
);
