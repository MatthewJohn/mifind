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
    const response = await api.post('/search', request)
    return response.data
  },

  getEntity: async (id: string): Promise<Entity> => {
    const response = await api.get(`/entity/${encodeURIComponent(id)}`)
    return response.data
  },

  getFilters: async (): Promise<FilterInfo> => {
    const response = await api.get('/filters')
    return response.data
  },

  getProviders: async (): Promise<Provider[]> => {
    const response = await api.get('/providers')
    return response.data
  },
}

export default api
