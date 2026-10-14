-- SCIM deprovisioning deactivates users instead of deleting them, so authorship survives.
ALTER TABLE users ADD COLUMN active boolean NOT NULL DEFAULT true;
