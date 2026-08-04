-- A title-only task.update used to send a task back to backlog.
--
-- v_task_latest picked ONE task.update row — the newest by ts — and read title,
-- body and status from it. `kb event --type=task.update` without --status writes
-- a row whose status is NULL, so once such a row became the newest, the status
-- COALESCE fell through to base.status ('backlog') and the task's real column
-- was lost. Seven such rows exist in the wild and three tasks had silently
-- dropped back to backlog.
--
-- The fix separates the two questions. Title and body come from the newest
-- update that carried them; status comes from the newest update that actually
-- set a status. A task.done event still forces 'done', as 0004 established.

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
            ELSE COALESCE(st.status, base.status, 'backlog')
        END AS status
FROM    events base
LEFT JOIN (
    SELECT u.ref_id, u.title, u.body,
           ROW_NUMBER() OVER (PARTITION BY u.ref_id ORDER BY u.ts DESC, u.id DESC) AS rn
    FROM   events u
    WHERE  u.type = 'task.update'
) upd ON upd.ref_id = base.id AND upd.rn = 1
LEFT JOIN (
    SELECT u.ref_id, u.status,
           ROW_NUMBER() OVER (PARTITION BY u.ref_id ORDER BY u.ts DESC, u.id DESC) AS rn
    FROM   events u
    WHERE  u.type = 'task.update' AND u.status IS NOT NULL
) st ON st.ref_id = base.id AND st.rn = 1
WHERE   base.type = 'task.new';
