-- name: ListRoadDistances :many
SELECT to_place_id, distance_m, duration_s, source
FROM road_distances
WHERE from_place_id = @from_id AND to_place_id = ANY(@to_ids::uuid[]);

-- name: UpsertRoadDistance :exec
INSERT INTO road_distances (from_place_id, to_place_id, distance_m, duration_s, source, updated_at)
VALUES (@from_id, @to_id, @distance_m, @duration_s, @source, now())
ON CONFLICT (from_place_id, to_place_id) DO UPDATE SET
    distance_m = EXCLUDED.distance_m,
    duration_s = EXCLUDED.duration_s,
    source     = EXCLUDED.source,
    updated_at = now();
