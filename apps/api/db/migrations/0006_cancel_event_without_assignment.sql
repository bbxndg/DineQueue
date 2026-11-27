-- F-08 (#53): a customer cancellation is audited in operation_events but has no
-- seating assignment (the ticket was never held). Every other action keeps a
-- mandatory assignment reference. Additive; existing rows already satisfy it.
ALTER TABLE operation_events
 ALTER COLUMN assignment_id DROP NOT NULL,
 ADD CONSTRAINT chk_operation_event_assignment CHECK (
 assignment_id IS NOT NULL
 OR (action='CUSTOMER_CANCEL' AND from_state='WAITING' AND to_state='CANCELLED' AND actor_id IS NOT NULL));
