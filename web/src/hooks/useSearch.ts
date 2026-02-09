import { useQuery } from '@tanstack/react-query'
import { searchApi } from '@/lib/api'
import type { SearchRequest } from '@/types/api'

export function useSearch(searchRequest: SearchRequest | null) {
  return useQuery({
    queryKey: ['search', searchRequest],
    queryFn: () => searchApi.search(searchRequest!),
    enabled: !!searchRequest, // Allow empty queries
    staleTime: 1000 * 60 * 5,
  })
}

export function useEntity(id: string | null) {
  return useQuery({
    queryKey: ['entity', id],
    queryFn: () => searchApi.getEntity(id!),
    enabled: !!id,
    staleTime: 1000 * 60 * 10,
  })
}

export function useFilters(searchQuery?: string) {
  return useQuery({
    queryKey: ['filters', searchQuery],
    queryFn: () => searchApi.getFilters(searchQuery),
    staleTime: 1000 * 60 * 5,
  })
}

export function useProviders() {
  return useQuery({
    queryKey: ['providers'],
    queryFn: () => searchApi.getProviders(),
    staleTime: 1000 * 60 * 5,
  })
}
