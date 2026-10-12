-- KYB-S44: a project's default board follows its workflow (new statuses become columns) until configured by hand.
ALTER TABLE boards ADD COLUMN follows_workflow boolean NOT NULL DEFAULT false;
