-- name: ListStayPrices :many
SELECT city_id, city_name, price_kop, category, source, basis, period FROM stay_prices ORDER BY city_id;

-- name: UpsertStayPrices :batchexec
INSERT INTO stay_prices (city_id, city_name, price_kop, category, source, basis, period, updated_at)
VALUES (@city_id, @city_name, @price_kop, @category, @source, @basis, @period, now())
ON CONFLICT (city_id) DO UPDATE SET
    city_name = EXCLUDED.city_name, price_kop = EXCLUDED.price_kop, category = EXCLUDED.category,
    source = EXCLUDED.source, basis = EXCLUDED.basis, period = EXCLUDED.period, updated_at = now();

-- name: DeleteStalePrices :exec
-- Города, которых нет в новом расчёте (убраны с карты), — удаляем.
DELETE FROM stay_prices WHERE updated_at < @started;
