import createClient from 'openapi-fetch'
import type { components, paths } from './schema'

// Типы и пути генерируются из api/openapi.yaml: npm run gen:api
export const api = createClient<paths>({ baseUrl: '/api/v1' })

type Schemas = components['schemas']
export type Place = Schemas['Place']
export type TripOption = Schemas['TripOption']
export type TripsResponse = Schemas['TripsResponse']
export type RoadRoute = Schemas['RoadRoute']
export type Poi = Schemas['Poi']
export type DriveTrip = Schemas['DriveTrip']
export type Guide = Schemas['Guide']
export type GuideListing = Schemas['GuideListing']
export type FuelType = Schemas['FuelType']
export type Money = Schemas['Money']
export type Problem = Schemas['Problem']
export type TripSearchParams = Schemas['TripSearchParams']
export type Favorite = Schemas['Favorite']
export type FavoritePick = Schemas['FavoritePick']
export type FavoriteSnapshot = Schemas['FavoriteSnapshot']
export type FavoriteCurrent = Schemas['FavoriteCurrent']
export type TripItem = Schemas['TripItem']
export type TripItemStage = Schemas['TripItemStage']
export type TripItemCreate = Schemas['TripItemCreate']
export type LinkPreview = Schemas['LinkPreview']
