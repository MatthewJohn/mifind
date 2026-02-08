import { useMemo } from 'react'
import { useSearchStore } from '@/stores/searchStore'
import { useSearch } from '@/hooks/useSearch'
import { SearchResults } from '@/components/search/SearchResults'
import { FilterSidebar } from '@/components/search/FilterSidebar'
import { EntityModal } from '@/components/entity/EntityModal'
import { SearchRequest } from '@/types/api'

export function SearchPage() {
  const { query, selectedEntity, setSelectedEntity, selectedTypes, filters } = useSearchStore()

  // Build search request from store state
  const searchRequest: SearchRequest | null = useMemo(() => {
    if (!query.trim()) return null

    const request: SearchRequest = {
      query: query.trim(),
      page: 1,
      perPage: 24,
    }

    // Add type filters
    if (selectedTypes.length > 0) {
      request.filters = [
        ...(request.filters || []),
        ...selectedTypes.map(type => ({
          key: 'type',
          operator: 'eq' as const,
          value: type,
        })),
      ]
    }

    // Add attribute filters
    if (filters.length > 0) {
      request.filters = [...(request.filters || []), ...filters]
    }

    return request
  }, [query, selectedTypes, filters])

  const { data, isLoading, error } = useSearch(searchRequest)

  const handleEntityClick = (entity: any) => {
    setSelectedEntity(entity)
  }

  return (
    <div className="flex gap-6">
      <FilterSidebar />

      <div className="flex-1 min-w-0">
        {query.trim() && (
          <div className="mb-4">
            <p className="text-sm text-gray-600">
              {isLoading ? (
                'Searching...'
              ) : error ? (
                <span className="text-red-600">Error searching</span>
              ) : data ? (
                <>
                  Found <span className="font-semibold">{data.total}</span> results
                  {data.executionTime && (
                    <span className="text-gray-400 ml-2">
                      ({data.executionTime.toFixed(0)}ms)
                    </span>
                  )}
                </>
              ) : null}
            </p>
          </div>
        )}

        <SearchResults
          entities={data?.results || []}
          loading={isLoading}
          onEntityClick={handleEntityClick}
        />

        {!query.trim() && (
          <div className="text-center py-16">
            <div className="text-gray-400 mb-4">
              <svg className="w-16 h-16 mx-auto" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
              </svg>
            </div>
            <h2 className="text-xl font-semibold text-gray-700 mb-2">Start searching</h2>
            <p className="text-gray-500">Enter a query above to search across all your content</p>
          </div>
        )}
      </div>

      <EntityModal
        entity={selectedEntity}
        open={!!selectedEntity}
        onOpenChange={(open) => !open && setSelectedEntity(null)}
      />
    </div>
  )
}
