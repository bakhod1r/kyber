-- ADR-0003: issue security levels ("" = visible to everyone who can browse the project).
ALTER TABLE issues ADD COLUMN security_level text NOT NULL DEFAULT '';
