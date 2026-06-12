-- Fix phantom backlog tasks: tasks that were closed via task.done before
-- migration 0003_status.sql introduced task.update status tracking.
-- These tasks have a task.done event but no task.update with status='done',
-- so v_task_latest reports them as 'backlog' instead of 'done'.
--
-- The fix: recreate v_task_latest so that the presence of any task.done event
-- forces status = 'done', taking precedence over task.update status.

DROP VIEW IF EXISTS v_task_latest;
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
