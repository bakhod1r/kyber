-- D3: planned start and due dates of an issue (the timeline). NULL = not set.
ALTER TABLE issues ADD COLUMN start_date date, ADD COLUMN due_date date;
