-- Add the 'note' event type.
--
-- A note is a standalone log entry: something that already happened. It carries
-- project/scope/title/body but has no status and no lifecycle, and it never
-- reaches the board -- v_open_tasks and v_task_latest both select type =
-- 'task.new', so notes are excluded by construction.
--
-- Splitting notes out of task.new is what keeps the backlog actionable. Without
-- this type, every "PR #829 shipped" or "migration proven broken" had to be
-- filed as a task; those entries were born in backlog and never closed, because
-- there was nothing left to do on them.
--
-- events.type is guarded by a CHECK constraint and SQLite cannot alter one in
-- place, so the table is rebuilt. The views are dropped first (they select from
-- events and would otherwise be rewritten by the RENAME) and recreated
-- afterwards exactly as 0004_done_status.sql left them.

PRAGMA foreign_keys = off;

DROP VIEW IF EXISTS v_open_tasks;
DROP VIEW IF EXISTS v_task_latest;

CREATE TABLE events_new (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ts          INTEGER NOT NULL,                  -- unix epoch seconds
    type        TEXT    NOT NULL CHECK (type IN (
                    'task.new', 'task.done', 'task.update',
                    'scope.shift', 'scope.expand', 'note'
                )),
    project     TEXT    NOT NULL,                  -- derived from cwd or explicit
    scope       TEXT,                              -- feature/change name, optional
    title       TEXT    NOT NULL,
    body        TEXT,                              -- full content with reasoning
    ref_id      INTEGER REFERENCES events_new(id), -- done/update pointing to the original task.new
    session_id  TEXT,                              -- Claude Code session, optional
    source      TEXT    NOT NULL DEFAULT 'manual', -- manual | hook-stop | hook-post | agent
    meta_json   TEXT,
    status      TEXT
);

INSERT INTO events_new
    (id, ts, type, project, scope, title, body, ref_id, session_id, source, meta_json, status)
SELECT
     id, ts, type, project, scope, title, body, ref_id, session_id, source, meta_json, status
FROM events;

DROP TABLE events;
ALTER TABLE events_new RENAME TO events;

CREATE INDEX IF NOT EXISTS idx_events_project_ts ON events(project, ts DESC);
CREATE INDEX IF NOT EXISTS idx_events_type_ts    ON events(type, ts DESC);
CREATE INDEX IF NOT EXISTS idx_events_scope      ON events(project, scope, ts);
CREATE INDEX IF NOT EXISTS idx_events_ref        ON events(ref_id);

CREATE VIEW v_open_tasks AS
SELECT e.*
FROM   events e
WHERE  e.type = 'task.new'
  AND  NOT EXISTS (
        SELECT 1 FROM events d
        WHERE  d.type   = 'task.done'
          AND  d.ref_id = e.id
  );

CREATE VIEW v_task_latest AS
SELECT  base.id                 AS id,
        base.ts                 AS created_ts,
        base.project,
        base.scope,
        COALESCE(upd.title, base.title) AS title,
        COALESCE(upd.body,  base.body)  AS body,
        base.source,
        CASE
            WHEN EXISTS (
                SELECT 1 FROM events d
                WHERE  d.type   = 'task.done'
                  AND  d.ref_id = base.id
            ) THEN 'done'
            ELSE COALESCE(upd.status, base.status, 'backlog')
        END AS status
FROM    events base
LEFT JOIN (
    SELECT u.ref_id, u.title, u.body, u.status,
           ROW_NUMBER() OVER (PARTITION BY u.ref_id ORDER BY u.ts DESC) AS rn
    FROM   events u
    WHERE  u.type = 'task.update'
) upd ON upd.ref_id = base.id AND upd.rn = 1
WHERE   base.type = 'task.new';

PRAGMA foreign_keys = on;
