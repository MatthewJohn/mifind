import { create } from 'zustand'
import type { Entity, SearchFacet, SearchFilter } from '@/types/api'

interface SearchStore {
  query: string
  setQuery: (query: string) => void

  currentPage: number
  setCurrentPage: (page: number) => void
  resultsPerPage: number

  selectedEntity: Entity | null
  setSelectedEntity: (entity: Entity | null) => void

  filters: SearchFilter[]
  setFilters: (filters: SearchFilter[]) => void
  addFilter: (filter: SearchFilter) => void
  addFilters: (newFilters: SearchFilter[]) => void
  removeFilter: (key: string) => void
  removeFiltersByKey: (key: string) => void
  clearFilters: () => void

  selectedTypes: string[]
  setSelectedTypes: (types: string[]) => void
  toggleType: (type: string) => void

  facets: SearchFacet[]
  setFacets: (facets: SearchFacet[]) => void

  isFilterOpen: boolean
  setFilterOpen: (open: boolean) => void
}

export const useSearchStore = create<SearchStore>((set) => ({
  query: '',
  setQuery: (query) => set({ query }),

  currentPage: 1,
  setCurrentPage: (page) => set({ currentPage: page }),
  resultsPerPage: 24,

  selectedEntity: null,
  setSelectedEntity: (entity) => set({ selectedEntity: entity }),

  filters: [],
  setFilters: (filters) => set({ filters }),
  addFilter: (filter) => set((state) => ({
    filters: [...state.filters.filter(f => f.key !== filter.key), filter]
  })),
  addFilters: (newFilters) => set((state) => ({
    filters: [...state.filters, ...newFilters]
  })),
  removeFilter: (key) => set((state) => ({
    filters: state.filters.filter(f => f.key !== key)
  })),
  removeFiltersByKey: (key) => set((state) => ({
    filters: state.filters.filter(f => f.key !== key)
  })),
  clearFilters: () => set({ filters: [] }),

  selectedTypes: [],
  setSelectedTypes: (types) => set({ selectedTypes: types }),
  toggleType: (type) => set((state) => {
    const types = state.selectedTypes.includes(type)
      ? state.selectedTypes.filter(t => t !== type)
      : [...state.selectedTypes, type]
    return { selectedTypes: types }
  }),

  facets: [],
  setFacets: (facets) => set({ facets }),

  isFilterOpen: false,
  setFilterOpen: (open) => set({ isFilterOpen: open }),
}))
