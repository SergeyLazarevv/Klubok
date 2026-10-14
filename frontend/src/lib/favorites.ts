import type { DriveTrip, FavoritePick, FavoriteSnapshot, Place, TripOption, TripSearchParams } from '@/api/client'
import { monthLabel } from '@/lib/format'

/**
 * Ключ варианта — та же строка, что favorites.Key на сервере: по ней карточка в результатах узнаёт,
 * что вариант уже в избранном. Настройки машины в ключ не входят: поменял расход — маршрут тот же.
 */
export function favoriteKey(p: TripSearchParams, pick: FavoritePick): string {
  return [
    p.from, p.to, p.month, p.min_days ?? 3, p.max_days ?? 14, p.travelers ?? 1, p.use_car ? 'car' : 'nocar',
    pick.kind, pick.hub_iata ?? '', pick.depart_date ?? '', pick.return_date ?? '',
  ].join('|')
}

/** Дата вылета и возвращения — по местному времени аэропорта, как в ответе API. */
export const flightPick = (o: TripOption): FavoritePick => ({
  kind: 'flight',
  hub_iata: o.hub.iata,
  depart_date: o.depart_at.slice(0, 10),
  return_date: o.return_at.slice(0, 10),
})

export const drivePick: FavoritePick = { kind: 'drive' }

export function flightSnapshot(o: TripOption): FavoriteSnapshot {
  return {
    total: o.total,
    per_person: o.per_person,
    hub_name: o.hub.city_name ?? o.hub.name,
    depart_at: o.depart_at,
    return_at: o.return_at,
    trip_days: o.trip_days,
    transfers: o.transfers,
    found_on: o.found_on,
  }
}

export function driveSnapshot(d: DriveTrip): FavoriteSnapshot {
  return { total: d.total, per_person: d.per_person, drive_hours: d.drive_hours, trip_days: d.trip_days }
}

/** «Самара → Алматы, ноябрь 2026» */
export const favoriteTitle = (home: Place, destination: Place, month: string) =>
  `${home.city_name ?? home.name} → ${destination.city_name ?? destination.name}, ${monthLabel(month)}`
