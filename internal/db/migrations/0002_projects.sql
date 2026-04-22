-- Project registry: maps a project name to a directory path on disk.
CREATE TABLE IF NOT EXISTS projects (
    name        TEXT PRIMARY KEY,
    path        TEXT NOT NULL UNIQUE,
    created_ts  INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_projects_path ON projects(path);
