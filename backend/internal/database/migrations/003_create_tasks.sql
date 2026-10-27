-- migrations/003_create_tasks.sql
CREATE TABLE tasks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    intention_id UUID NOT NULL REFERENCES intentions(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT,
    deadline TIMESTAMP WITH TIME ZONE,
    estimated_duration INT, -- minutes
    priority INT DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT now(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT now()
);
