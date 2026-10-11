-- name: GetFuelPrice :one
-- Цена по региону, если есть, иначе по стране.
SELECT price_kop
FROM fuel_prices
WHERE country_code = @country_code AND fuel_type = @fuel_type AND region_code IN (@region_code::text, '')
ORDER BY region_code DESC
LIMIT 1;

-- name: ListParking :many
SELECT place_id, parking_per_day_kop
FROM hub_costs
WHERE place_id = ANY(@place_ids::uuid[]);

-- name: ListBorderRules :many
SELECT to_country, insurance_kop, crossing_hours::float8 AS crossing_hours, note
FROM border_rules
WHERE from_country = @from_country;
