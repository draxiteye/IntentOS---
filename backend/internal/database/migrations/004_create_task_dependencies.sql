-- migrations/004_create_task_dependencies.sql
CREATE TABLE task_dependencies (
    task_id UUID NOT NULL,
    depends_on_task_id UUID NOT NULL,
    PRIMARY KEY (task_id, depends_on_task_id),
    CONSTRAINT fk_task FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
    CONSTRAINT fk_depends_on_task FOREIGN KEY (depends_on_task_id) REFERENCES tasks(id) ON DELETE CASCADE,
    CONSTRAINT chk_no_self_dependency CHECK (task_id <> depends_on_task_id)
);

-- Index for fast lookup of dependencies
CREATE INDEX idx_task_dependencies_task ON task_dependencies(task_id);
CREATE INDEX idx_task_dependencies_dep ON task_dependencies(depends_on_task_id);
