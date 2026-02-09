import { useMemo, useCallback, useEffect } from 'react'
import { useSearchStore } from '@/stores/searchStore'
import { useSearch } from '@/hooks/useSearch'
import { SearchResults } from '@/components/search/SearchResults'
import { FilterSidebar } from '@/components/search/FilterSidebar'
import { EntityModal } from '@/components/entity/EntityModal'
import { Pagination } from '@/components/search/Pagination'
import { SearchRequest } from '@/types/api'

export function SearchPage() {
  const {
    query,
    searchTriggered,
    resetSearchTrigger,
    currentPage,
    setCurrentPage,
    resultsPerPage,
    selectedEntity,
    setSelectedEntity,
    selectedTypes,
    filters
  } = useSearchStore()

  // Reset to page 1 when filters change (but not when just query changes)
  useEffect(() => {
    setCurrentPage(1)
  }, [selectedTypes, filters, setCurrentPage])

  // Build search request from store state (only when search is triggered)
  const searchRequest: SearchRequest | null = useMemo(() => {
    if (!searchTriggered) return null

    const request: SearchRequest = {
      query: query.trim(), // Allow empty queries
      page: currentPage,
      perPage: resultsPerPage,
    }

    // Add type filters
    if (selectedTypes.length > 0) {
      request.filters = [
        ...selectedTypes.map(type => ({
          key: 'type',
          operator: 'eq' as const,
          value: type,
        })),
      ]
    }

    // Add attribute filters
    if (filters.length > 0) {
      request.filters = [
        ...(request.filters || []),
        ...filters
      ]
    }

    // Reset the trigger after building the request
    resetSearchTrigger()

    return request
  }, [searchTriggered, query, currentPage, resultsPerPage, selectedTypes, filters, resetSearchTrigger])

  const { data, isLoading, error } = useSearch(searchRequest)

  const handleEntityClick = useCallback((entity: any) => {
    setSelectedEntity(entity)
  }, [setSelectedEntity])

  const handlePageChange = useCallback((page: number) => {
    setCurrentPage(page)
    // Scroll to top of results
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }, [setCurrentPage])

  const totalPages = data ? Math.ceil(data.total_count / resultsPerPage) : 0

  // Show "no search" state when search hasn't been triggered yet
  const hasSearched = searchTriggered || data

  return (
    <div className="flex gap-6">
      <FilterSidebar />

      <div className="flex-1 min-w-0">
        {hasSearched && (
          <div className="mb-4">
            <p className="text-sm text-gray-600">
              {isLoading ? (
                'Searching...'
              ) : error ? (
                <span className="text-red-600">Error searching</span>
              ) : data ? (
                <>
                  Found <span className="font-semibold">{data.total_count}</span> results
                  {query && (
                    <span className="text-gray-400 ml-2">
                      for "{query}"
                    </span>
                  )}
                  {data.duration_ms && (
                    <span className="text-gray-400 ml-2">
                      ({data.duration_ms.toFixed(0)}ms)
                    </span>
                  )}
                  {totalPages > 1 && (
                    <span className="text-gray-400 ml-2">
                      Page {currentPage} of {totalPages}
                    </span>
                  )}
                </>
              ) : null}
            </p>
          </div>
        )}

        <SearchResults
          entities={data?.entities || []}
          loading={isLoading}
          onEntityClick={handleEntityClick}
        />

        {/* Pagination */}
        {data && data.total_count > 0 && (
          <div className="mt-6">
            <Pagination
              currentPage={currentPage}
              totalPages={totalPages}
              totalResults={data.total_count}
              resultsPerPage={resultsPerPage}
              onPageChange={handlePageChange}
              isLoading={isLoading}
            />
          </div>
        )}

        {!hasSearched && (
          <div className="text-center py-16">
            <div className="text-gray-400 mb-4">
              <svg className="w-16 h-16 mx-auto" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
              </svg>
            </div>
            <h2 className="text-xl font-semibold text-gray-700 mb-2">Start searching</h2>
            <p className="text-gray-500">Enter a query or set filters, then click Search</p>
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
