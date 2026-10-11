import createClient from 'openapi-fetch'
import type { components, paths } from './schema'

// Типы и пути генерируются из api/openapi.yaml: npm run gen:api
export const api = createClient<paths>({ baseUrl: '/api/v1' })

type Schemas = components['schemas']
export type Place = Schemas['Place']
export type TripOption = Schemas['TripOption']
export type TripsResponse = Schemas['TripsResponse']
export type FuelType = Schemas['FuelType']
export type Money = Schemas['Money']
export type Problem = Schemas['Problem']
