-- name: SearchHouses :many
SELECT * FROM houses
WHERE NOT EXISTS (
    SELECT 1
    FROM unnest(string_to_array(trim(regexp_replace(@query::text, '[,.\-]+', ' ', 'g')), ' ')) AS w
    WHERE w <> '' AND address NOT ILIKE '%' || w || '%'
)
ORDER BY address
LIMIT 20;

-- name: NearestHouses :many
-- Плоская аппроксимация расстояния: для радиуса в пару километров точности хватает.
-- Дома без координат (импорт, где геокодер не нашёл адрес) в выдачу не попадают.
SELECT * FROM houses
WHERE NOT (lat = 0 AND lon = 0)
ORDER BY (lat - @lat::float8) ^ 2 + ((lon - @lon::float8) * cos(radians(@lat::float8))) ^ 2
LIMIT @max_rows;

-- name: GetHouse :one
SELECT * FROM houses WHERE id = @id;

-- name: GetOrganization :one
SELECT * FROM organizations WHERE id = @id;

-- name: UpsertOrganization :exec
INSERT INTO organizations (id, type, name, phone_office, phone_dispatcher, phone_emergency, schedule, source)
VALUES (@id, @type, @name, @phone_office, @phone_dispatcher, @phone_emergency, @schedule, @source)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    phone_office = EXCLUDED.phone_office,
    phone_dispatcher = EXCLUDED.phone_dispatcher,
    phone_emergency = EXCLUDED.phone_emergency,
    schedule = EXCLUDED.schedule;

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

-- name: ListDistrictOrganizations :many
SELECT DISTINCT o.* FROM organizations o
JOIN houses h ON h.organization_id = o.id
WHERE h.district = @district
ORDER BY o.name;

-- name: UpsertHouse :one
-- Импорт реестра: дом обновляется по id. xmax = 0 только у только что вставленной строки.
INSERT INTO houses (id, address, district, year_built, floors, entrances_count, organization_id, lat, lon, source)
VALUES (@id, @address, @district, @year_built, @floors, @entrances_count, @organization_id, @lat, @lon, @source)
ON CONFLICT (id) DO UPDATE SET
    address = EXCLUDED.address, district = EXCLUDED.district, year_built = EXCLUDED.year_built,
    floors = EXCLUDED.floors, entrances_count = EXCLUDED.entrances_count,
    organization_id = EXCLUDED.organization_id, lat = EXCLUDED.lat, lon = EXCLUDED.lon, source = EXCLUDED.source
RETURNING (xmax = 0)::bool AS created;

-- name: EnsureEntrances :exec
-- Подъезды 1..N; лишние старые не удаляются: на них могут ссылаться заявки.
INSERT INTO entrances (id, house_id, number)
SELECT @house_id::text || '-e' || n, @house_id::text, n
FROM generate_series(1, @entrances_count::int) AS n
ON CONFLICT (id) DO NOTHING;

-- name: EnsureHouseObjects :exec
-- Те же объекты и коды, что в модельных данных (миграция 00002): лифт и свет в подъезде, кровля, мусоропровод.
INSERT INTO asset_objects (id, house_id, entrance_id, category, label, qr_code)
SELECT e.id || '-lift', e.house_id, e.id, 'lift', 'подъезд ' || e.number || ', пассажирский лифт', e.id || '-lift'
FROM entrances e WHERE e.house_id = @house_id
UNION ALL
SELECT e.id || '-light', e.house_id, e.id, 'lighting', 'подъезд ' || e.number || ', лестничная клетка', e.id || '-light'
FROM entrances e WHERE e.house_id = @house_id
UNION ALL
SELECT @house_id::text || '-roof', @house_id::text, NULL, 'leak', 'кровля', @house_id::text || '-roof'
UNION ALL
SELECT @house_id::text || '-trash', @house_id::text, NULL, 'garbage', 'мусоропровод', @house_id::text || '-trash'
ON CONFLICT (id) DO NOTHING;
