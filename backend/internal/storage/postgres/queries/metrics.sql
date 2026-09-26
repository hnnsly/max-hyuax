-- name: ListFirstResponses :many
SELECT i.created_at, min(e.at)::timestamptz AS responded_at
FROM issues i
JOIN issue_events e ON e.issue_id = i.id AND e.kind = 'status_changed'
WHERE i.responsible_org_id = @org_id AND i.created_at >= @since
GROUP BY i.id, i.created_at;

-- name: OrgMetricCounts :one
SELECT count(*) FILTER (WHERE i.created_at >= @since)::int AS issues,
       COALESCE(sum(p.n) FILTER (WHERE i.created_at >= @since), 0)::int AS reports,
       count(*) FILTER (WHERE i.status IN ('done', 'rejected') AND i.status_at >= @since)::int AS closed_total,
       count(*) FILTER (WHERE i.status IN ('done', 'rejected') AND i.status_at >= @since
                          AND i.status_at <= i.deadline_at)::int AS closed_on_time,
       count(*) FILTER (WHERE i.status = 'done' AND i.status_at >= @since AND EXISTS (
                            SELECT 1 FROM issue_confirmations c
                            WHERE c.issue_id = i.id AND c.done_at = i.status_at AND c.fixed))::int AS confirmed,
       count(*) FILTER (WHERE i.reopened_at >= @since)::int AS reopened,
       count(*) FILTER (WHERE i.status NOT IN ('done', 'rejected'))::int AS open_total,
       count(*) FILTER (WHERE i.status NOT IN ('done', 'rejected') AND i.deadline_at < @now)::int AS overdue_open,
       COALESCE(sum(r.s), 0)::int AS rating_sum,
       COALESCE(sum(r.n), 0)::int AS ratings,
       COALESCE(bool_or(i.sample), false)::boolean AS sample_data
FROM issues i
LEFT JOIN LATERAL (SELECT count(*) AS n FROM issue_participants ip WHERE ip.issue_id = i.id) p ON true
-- Оценки ремонтов, отмеченных выполненными за период (ADR-022).
LEFT JOIN LATERAL (SELECT sum(c.stars) AS s, count(c.stars) AS n
                   FROM issue_confirmations c
                   WHERE c.issue_id = i.id AND c.done_at >= @since) r ON true
WHERE i.responsible_org_id = @org_id;

-- name: LockSampleShift :exec
-- Сдвиг примера данных идёт под блокировкой до конца транзакции: два экземпляра api не сдвинут дважды.
SELECT pg_advisory_xact_lock(7340301);

-- name: SampleLatestAt :one
-- Самое позднее событие примера данных (срок не в счёт: он может быть в будущем).
SELECT COALESCE(max(t), @now::timestamptz)::timestamptz AS latest
FROM (
    SELECT greatest(i.created_at, i.status_at, i.overdue_at, i.reopened_at) AS t FROM issues i WHERE i.sample
    UNION ALL
    SELECT c.at FROM issue_confirmations c JOIN issues i ON i.id = c.issue_id WHERE i.sample
    UNION ALL
    SELECT e.at FROM issue_events e JOIN issues i ON i.id = e.issue_id WHERE i.sample
    UNION ALL
    SELECT p.joined_at FROM issue_participants p JOIN issues i ON i.id = p.issue_id WHERE i.sample
) AS x;

-- name: ShiftSampleIssues :exec
UPDATE issues
SET created_at  = created_at + make_interval(days => @days::int),
    status_at   = status_at + make_interval(days => @days::int),
    deadline_at = deadline_at + make_interval(days => @days::int),
    overdue_at  = overdue_at + make_interval(days => @days::int),
    reopened_at = reopened_at + make_interval(days => @days::int)
WHERE sample;

-- name: ShiftSampleConfirmations :exec
-- done_at сдвигается вместе со status_at, иначе ответы перестанут относиться к текущему «выполнено».
UPDATE issue_confirmations c
SET done_at = c.done_at + make_interval(days => @days::int),
    at      = c.at + make_interval(days => @days::int)
FROM issues i
WHERE i.id = c.issue_id AND i.sample;

-- name: ShiftSampleEvents :exec
UPDATE issue_events e
SET at = e.at + make_interval(days => @days::int)
FROM issues i
WHERE i.id = e.issue_id AND i.sample;

-- name: ShiftSampleParticipants :exec
UPDATE issue_participants p
SET joined_at = p.joined_at + make_interval(days => @days::int)
FROM issues i
WHERE i.id = p.issue_id AND i.sample;
