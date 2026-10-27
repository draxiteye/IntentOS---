-- migrations/008_enhance_intentions_and_tasks.sql
ALTER TABLE intentions ADD COLUMN IF NOT EXISTS horizon TEXT DEFAULT 'Mastery';

ALTER TABLE tasks ADD COLUMN IF NOT EXISTS energy_level TEXT DEFAULT 'medium';
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS actual_duration INT DEFAULT 0;

ALTER TABLE plan_revisions ADD COLUMN IF NOT EXISTS metadata JSONB DEFAULT '{}'::jsonb;
