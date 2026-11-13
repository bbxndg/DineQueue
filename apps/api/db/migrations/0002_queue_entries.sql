-- F-02: daily ticket allocator and walk-in queue entries (BR-001, BR-002, BR-008 v2, BR-009).
CREATE TABLE queue_day_counters (
 queue_id uuid NOT NULL REFERENCES service_queues(id) ON DELETE CASCADE,
 service_date date NOT NULL,
 last_number integer NOT NULL DEFAULT 0 CHECK (last_number >= 0),
 PRIMARY KEY (queue_id, service_date)
);
CREATE TABLE queue_entries (
 id uuid PRIMARY KEY,
 queue_id uuid NOT NULL REFERENCES service_queues(id) ON DELETE RESTRICT,
 customer_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 service_date date NOT NULL,
 sequence_number integer NOT NULL CHECK (sequence_number > 0),
 party_size integer NOT NULL CONSTRAINT chk_party_size CHECK (party_size BETWEEN 1 AND 16),
 status text NOT NULL DEFAULT 'WAITING' CHECK (status IN ('WAITING', 'CALLED', 'COMPLETED')),
 joined_at timestamptz NOT NULL,
 called_at timestamptz,
 called_by uuid REFERENCES users(id) ON DELETE RESTRICT,
 completed_at timestamptz,
 completed_by uuid REFERENCES users(id) ON DELETE RESTRICT,
 version integer NOT NULL DEFAULT 1 CHECK (version >= 1),
 CONSTRAINT chk_queue_entry_lifecycle CHECK (
  (status = 'WAITING' AND called_at IS NULL AND called_by IS NULL AND completed_at IS NULL AND completed_by IS NULL)
  OR (status = 'CALLED' AND called_at IS NOT NULL AND called_by IS NOT NULL AND completed_at IS NULL AND completed_by IS NULL)
  OR (status = 'COMPLETED' AND called_at IS NOT NULL AND called_by IS NOT NULL AND completed_at IS NOT NULL AND completed_by IS NOT NULL)
 ),
 CHECK (called_at IS NULL OR called_at >= joined_at),
 CHECK (completed_at IS NULL OR completed_at >= called_at)
);
-- One displayed ticket number per queue per local service day.
CREATE UNIQUE INDEX uq_queue_day_number ON queue_entries (queue_id, service_date, sequence_number);
-- BR-001: one active (WAITING/CALLED) entry per customer per queue, even under races.
CREATE UNIQUE INDEX uq_one_active_ticket_per_customer_queue ON queue_entries (customer_id, queue_id) WHERE status IN ('WAITING', 'CALLED');
-- S1 one currently called entry per queue (enforced for F-04 Call Next).
CREATE UNIQUE INDEX uq_one_called_per_queue ON queue_entries (queue_id) WHERE status = 'CALLED';
CREATE INDEX idx_queue_fifo ON queue_entries (queue_id, status, joined_at, id);
