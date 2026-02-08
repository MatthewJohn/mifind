import axios from 'axios'
import type { Entity, SearchResult, FilterInfo, Provider, SearchRequest } from '@/types/api'

const API_BASE_URL = import.meta.env.VITE_API_URL || '/api'

const api = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
})

export const searchApi = {
  search: async (request: SearchRequest): Promise<SearchResult> => {
    // Transform request to match Go API format
    const goRequest: any = {
      query: request.query,
      limit: request.perPage || 20,
      offset: request.page ? ((request.page - 1) * (request.perPage || 20)) : 0,
    }

    // Transform filters array to map format
    if (request.filters && request.filters.length > 0) {
      goRequest.filters = {}
      for (const filter of request.filters) {
        // Use simple key-value format for now
        goRequest.filters[filter.key] = filter.value
      }
    }

    const response = await api.post('/search', goRequest)
    return response.data
  },

  getEntity: async (id: string): Promise<Entity> => {
    const response = await api.get(`/entity/${encodeURIComponent(id)}`)
    return response.data
  },

  getFilters: async (searchQuery?: string): Promise<FilterInfo> => {
    const params = searchQuery ? { search: searchQuery } : {}
    const response = await api.get('/filters', { params })
    return response.data
  },

  getProviders: async (): Promise<Provider[]> => {
    const response = await api.get('/providers')
    return response.data
  },
}

export default api
