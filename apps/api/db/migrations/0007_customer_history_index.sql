-- S3 #56 Customer Ticket History: owner + terminal + (joined_at, id) keyset reads.
-- Justified by an isolated populated-data plan (200k rows): without it the read is a
-- parallel sequential scan of all queue_entries; with it an index scan of the owner's
-- terminal rows that also carries the keyset boundary. Additive; no data changes.
CREATE INDEX idx_queue_entries_customer_history
 ON queue_entries (customer_id, joined_at DESC, id DESC)
 WHERE status IN ('COMPLETED','CANCELLED','NO_SHOW');
