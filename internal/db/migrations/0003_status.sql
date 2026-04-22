-- Add status column to events.
-- Status values: backlog, in_progress, testing, complete, done
ALTER TABLE events ADD COLUMN status TEXT;

-- Backfill existing rows
UPDATE events SET status = 'backlog' WHERE type = 'task.new' AND status IS NULL;
UPDATE events SET status = 'done' WHERE type = 'task.done' AND status IS NULL;

-- Recreate views to include status

DROP VIEW IF EXISTS v_open_tasks;
CREATE VIEW v_open_tasks AS
SELECT e.*
FROM   events e
WHERE  e.type = 'task.new'
  AND  NOT EXISTS (
        SELECT 1 FROM events d
        WHERE  d.type   = 'task.done'
          AND  d.ref_id = e.id
  );

DROP VIEW IF EXISTS v_task_latest;
CREATE VIEW v_task_latest AS
SELECT  base.id              AS id,
        base.ts              AS created_ts,
        base.project,
        base.scope,
        COALESCE(upd.title, base.title) AS title,
        COALESCE(upd.body,  base.body)  AS body,
        base.source,
        COALESCE(upd.status, base.status, 'backlog') AS status
FROM    events base
LEFT JOIN (
    SELECT u.ref_id, u.title, u.body, u.status,
           ROW_NUMBER() OVER (PARTITION BY u.ref_id ORDER BY u.ts DESC) AS rn
    FROM   events u
    WHERE  u.type = 'task.update'
) upd ON upd.ref_id = base.id AND upd.rn = 1
WHERE   base.type = 'task.new';
