-- migrations/005_create_user_availability.sql
CREATE TABLE user_availability (
    user_id UUID NOT NULL,
    weekday SMALLINT NOT NULL CHECK (weekday BETWEEN 0 AND 6), -- 0=Sunday .. 6=Saturday
    start_time TIME NOT NULL,
    end_time TIME NOT NULL,
    PRIMARY KEY (user_id, weekday, start_time)
);

-- Index for quick lookup per user
CREATE INDEX idx_user_availability_user ON user_availability(user_id);
