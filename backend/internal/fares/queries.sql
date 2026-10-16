-- name: SaveQuotes :batchexec
INSERT INTO price_quotes (source, origin_iata, destination_iata, depart_at, return_at, price_kop,
                          transfers, return_transfers, airline, found_on)
VALUES (@source, @origin_iata, @destination_iata, @depart_at, @return_at, @price_kop,
        @transfers, @return_transfers, @airline, @found_on)
ON CONFLICT DO NOTHING;

-- name: GetFareSnapshot :one
SELECT fetched_at, quotes FROM fare_snapshots
WHERE source = @source AND origin_iata = @origin_iata AND month = @month;

-- name: UpsertFareSnapshot :exec
INSERT INTO fare_snapshots (source, origin_iata, month, fetched_at, quotes)
VALUES (@source, @origin_iata, @month, now(), @quotes)
ON CONFLICT (source, origin_iata, month) DO UPDATE SET fetched_at = now(), quotes = EXCLUDED.quotes;
