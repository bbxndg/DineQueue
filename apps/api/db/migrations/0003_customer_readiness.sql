-- Lab 7 #44: preserve eligibility of existing tickets, require new parties to opt in.
ALTER TABLE queue_entries ADD COLUMN is_ready boolean NOT NULL DEFAULT true;
ALTER TABLE queue_entries ALTER COLUMN is_ready SET DEFAULT false;
