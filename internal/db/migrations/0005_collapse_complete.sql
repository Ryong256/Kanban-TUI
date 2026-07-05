-- Collapse the 'complete' column into 'done'.
--
-- 'complete' and 'done' were two terminal columns; agents stopped at 'complete'
-- and tasks never reached 'done'. The board now has four columns:
-- backlog -> in_progress -> testing -> done.
--
-- v_task_latest reports a task as 'done' when a task.done event exists for it,
-- and the open-task views exclude tasks by NOT EXISTS task.done. So renaming the
-- status string is not enough: we append a task.done event for every task whose
-- current status is 'complete' (mirrors MoveTask's behavior for done), keeping the
-- board, `kb list`, and `kb count` consistent.
INSERT INTO events (ts, type, project, title, ref_id, source, status)
SELECT strftime('%s', 'now'), 'task.done', t.project, t.title, t.id, 'migration', 'done'
FROM   v_task_latest t
WHERE  t.status = 'complete';
