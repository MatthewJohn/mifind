// API Types matching Go structs

export interface Entity {
  ID: string
  Type: string
  Title: string
  Description?: string
  Timestamp: string  // ISO timestamp string
  Attributes: Record<string, any>
  Relationships?: Relationship[]
  SearchTokens?: string[]
  Thumbnail?: string
  Provider?: string
  score?: number
}

export interface Relationship {
  Type: string
  TargetID: string
}

export interface EntityAttribute {
  key: string
  value: string
  type?: string
  weight?: number
}

export interface SearchResult {
  entities: Entity[]
  total_count: number
  type_counts: Record<string, number>
  duration_ms: number
  query?: string
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
  capabilities: Record<string, any>
  filters?: FilterResult
  values?: Record<string, FilterOption[]>
}

export interface FilterResult {
  TypeCounts: Record<string, number>
  AttributeCounts: Record<string, Record<string, number>>
}
