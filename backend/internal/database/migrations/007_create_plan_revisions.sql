-- migrations/007_create_plan_revisions.sql
CREATE TABLE plan_revisions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT now(),
    description TEXT
);

CREATE INDEX idx_plan_revisions_user ON plan_revisions(user_id);
