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
       count(*) FILTER (WHERE i.status NOT IN ('done', 'rejected'))::int AS open_total,
       count(*) FILTER (WHERE i.status NOT IN ('done', 'rejected') AND i.deadline_at < @now)::int AS overdue_open,
       COALESCE(bool_or(i.sample), false)::boolean AS sample_data
FROM issues i
LEFT JOIN LATERAL (SELECT count(*) AS n FROM issue_participants ip WHERE ip.issue_id = i.id) p ON true
WHERE i.responsible_org_id = @org_id;

-- name: SampleLatestAt :one
-- Самое позднее событие примера данных (срок не в счёт: он может быть в будущем).
SELECT COALESCE(max(t), @now::timestamptz)::timestamptz AS latest
FROM (
    SELECT greatest(i.created_at, i.status_at, i.overdue_at) AS t FROM issues i WHERE i.sample
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
    overdue_at  = overdue_at + make_interval(days => @days::int)
WHERE sample;

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
