-- name: SearchHouses :many
SELECT * FROM houses
WHERE address ILIKE '%' || @query::text || '%'
ORDER BY address
LIMIT 20;

-- name: NearestHouses :many
-- Плоская аппроксимация расстояния: для радиуса в пару километров точности хватает.
SELECT * FROM houses
ORDER BY (lat - @lat::float8) ^ 2 + ((lon - @lon::float8) * cos(radians(@lat::float8))) ^ 2
LIMIT @max_rows;

-- name: GetHouse :one
SELECT * FROM houses WHERE id = @id;

-- name: GetOrganization :one
SELECT * FROM organizations WHERE id = @id;

-- name: ListEntrances :many
SELECT * FROM entrances WHERE house_id = @house_id ORDER BY number;

-- name: ListHouseObjects :many
SELECT id, house_id, COALESCE(entrance_id, '')::text AS entrance_id, category, label, qr_code
FROM asset_objects
WHERE house_id = @house_id
ORDER BY id;

-- name: GetObjectByCode :one
SELECT id, house_id, COALESCE(entrance_id, '')::text AS entrance_id, category, label, qr_code
FROM asset_objects
WHERE qr_code = @qr_code;

-- name: ListOrganizationHouses :many
SELECT * FROM houses WHERE organization_id = @organization_id ORDER BY address;
