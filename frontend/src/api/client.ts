import createClient from 'openapi-fetch'
import type { components, paths } from './schema'

// Типы и пути генерируются из api/openapi.yaml: npm run gen:api
export const api = createClient<paths>({ baseUrl: '/api/v1' })

export type Place = components['schemas']['Place']
export type Hub = components['schemas']['Hub']
