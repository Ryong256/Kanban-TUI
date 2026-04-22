-- Append-only event log. State is reconstructed from these rows.
CREATE TABLE IF NOT EXISTS events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ts          INTEGER NOT NULL,                  -- unix epoch seconds
    type        TEXT    NOT NULL CHECK (type IN (
                    'task.new', 'task.done', 'task.update',
                    'scope.shift', 'scope.expand'
                )),
    project     TEXT    NOT NULL,                  -- derived from cwd or explicit
    scope       TEXT,                              -- feature/change name, optional
    title       TEXT    NOT NULL,
    body        TEXT,                              -- full content with reasoning
    ref_id      INTEGER REFERENCES events(id),    -- for done/update pointing to original task.new
    session_id  TEXT,                              -- Claude Code session, optional
    source      TEXT    NOT NULL DEFAULT 'manual',-- manual | hook-stop | hook-post | agent
    meta_json   TEXT
);

CREATE INDEX IF NOT EXISTS idx_events_project_ts ON events(project, ts DESC);
CREATE INDEX IF NOT EXISTS idx_events_type_ts    ON events(type, ts DESC);
CREATE INDEX IF NOT EXISTS idx_events_scope      ON events(project, scope, ts);
CREATE INDEX IF NOT EXISTS idx_events_ref        ON events(ref_id);

-- Open tasks: task.new rows whose id has no matching task.done.
CREATE VIEW IF NOT EXISTS v_open_tasks AS
SELECT e.*
FROM   events e
WHERE  e.type = 'task.new'
  AND  NOT EXISTS (
        SELECT 1 FROM events d
        WHERE  d.type   = 'task.done'
          AND  d.ref_id = e.id
  );

-- Latest title per task (applies task.update overrides).
CREATE VIEW IF NOT EXISTS v_task_latest AS
SELECT  base.id              AS id,
        base.ts               AS created_ts,
        base.project,
        base.scope,
        COALESCE(upd.title, base.title) AS title,
        COALESCE(upd.body,  base.body)  AS body,
        base.source
FROM    events base
LEFT JOIN (
    SELECT u.ref_id, u.title, u.body,
           ROW_NUMBER() OVER (PARTITION BY u.ref_id ORDER BY u.ts DESC) AS rn
    FROM   events u
    WHERE  u.type = 'task.update'
) upd ON upd.ref_id = base.id AND upd.rn = 1
WHERE   base.type = 'task.new';
