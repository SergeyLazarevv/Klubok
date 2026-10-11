-- name: SaveQuotes :batchexec
INSERT INTO price_quotes (source, origin_iata, destination_iata, depart_at, return_at, price_kop,
                          transfers, return_transfers, airline, found_on)
VALUES (@source, @origin_iata, @destination_iata, @depart_at, @return_at, @price_kop,
        @transfers, @return_transfers, @airline, @found_on)
ON CONFLICT DO NOTHING;
