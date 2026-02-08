// API Types matching Go structs

export interface Entity {
  id: string
  type: string
  title: string
  description?: string
  timestamp: number
  attributes: EntityAttribute[]
  metadata?: Record<string, any>
  thumbnail?: string
  provider?: string
}

export interface EntityAttribute {
  key: string
  value: string
  type?: string
  weight?: number
}

export interface SearchResult {
  query: string
  results: Entity[]
  total: number
  page: number
  perPage: number
  facets?: SearchFacet[]
  executionTime: number
}

export interface SearchFacet {
  key: string
  values: FacetValue[]
}

export interface FacetValue {
  value: string
  count: number
}

export interface FilterOption {
  key: string
  label: string
  type: 'text' | 'select' | 'multiselect' | 'number' | 'date'
  options?: FilterOptionValue[]
  min?: number
  max?: number
}

export interface FilterOptionValue {
  value: string
  label: string
  count?: number
}

export interface Provider {
  id: string
  name: string
  type: string
  enabled: boolean
  entityCount?: number
}

export interface SearchRequest {
  query: string
  filters?: SearchFilter[]
  page?: number
  perPage?: number
  sortBy?: string
  sortOrder?: 'asc' | 'desc'
}

export interface SearchFilter {
  key: string
  operator: 'eq' | 'ne' | 'gt' | 'lt' | 'gte' | 'lte' | 'contains' | 'startsWith' | 'endsWith'
  value: string | number | boolean
}

export interface FilterInfo {
  attributeFilters: FilterOption[]
  typeFilters: string[]
}
