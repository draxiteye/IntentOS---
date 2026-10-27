-- migrations/006_create_schedule_blocks.sql
CREATE TABLE schedule_blocks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL,
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    start_timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    end_timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    CONSTRAINT chk_start_before_end CHECK (start_timestamp < end_timestamp)
);

-- Index for quick lookup by user and by task
CREATE INDEX idx_schedule_blocks_user ON schedule_blocks(user_id);
CREATE INDEX idx_schedule_blocks_task ON schedule_blocks(task_id);
