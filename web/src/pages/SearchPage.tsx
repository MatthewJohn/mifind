import { useMemo, useCallback, useEffect, useRef } from 'react'
import { useSearchStore } from '@/stores/searchStore'
import { useSearch } from '@/hooks/useSearch'
import { useSearchSync } from '@/hooks/useSearchSync'
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
    filters,
    triggerSearch,
  } = useSearchStore()

  // Sync search state with URL params (enables refresh and sharing)
  useSearchSync()

  // Track if we've ever triggered a search (to know when to show results vs "start searching")
  const hasEverSearchedRef = useRef(false)

  // Auto-trigger search when filters change (after initial search)
  const prevFiltersRef = useRef<typeof filters>([])
  const prevTypesRef = useRef<typeof selectedTypes>([])

  useEffect(() => {
    // Skip on first render or before first search
    if (!hasEverSearchedRef.current) return

    // Check if filters actually changed
    const filtersChanged = JSON.stringify(prevFiltersRef.current) !== JSON.stringify(filters)
    const typesChanged = JSON.stringify(prevTypesRef.current) !== JSON.stringify(selectedTypes)

    if (filtersChanged || typesChanged) {
      // Reset to page 1 and trigger search
      setCurrentPage(1)
      triggerSearch()
    }

    // Update refs
    prevFiltersRef.current = filters
    prevTypesRef.current = selectedTypes
  }, [filters, selectedTypes, setCurrentPage, triggerSearch])

  // Build search request from store state
  // IMPORTANT: query is NOT in dependencies - typing should NOT trigger search
  const searchRequest: SearchRequest | null = useMemo(() => {
    // Only build request when search is explicitly triggered
    if (!searchTriggered) return null

    // Mark that we've searched (persists for the session)
    hasEverSearchedRef.current = true

    const request: SearchRequest = {
      query: query.trim(),
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

    return request
  }, [searchTriggered, currentPage, resultsPerPage, selectedTypes, filters])

  // Reset search trigger after building request (it's been captured by keepSearchingRef)
  useEffect(() => {
    if (searchTriggered) {
      resetSearchTrigger()
    }
  }, [searchTriggered, resetSearchTrigger])

  // Run the search query
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

  // Simple state derivation
  const hasSearched = hasEverSearchedRef.current
  const hasResults = data?.entities && data.entities.length > 0

  return (
    <div className="flex gap-6">
      <FilterSidebar searchResult={data || undefined} />

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

        {hasSearched ? (
          <>
            {isLoading ? (
              <div className="flex justify-center py-12">
                <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-[#0654ba]"></div>
              </div>
            ) : hasResults ? (
              <SearchResults
                entities={data.entities}
                loading={false}
                onEntityClick={handleEntityClick}
              />
            ) : (
              <div className="text-center py-12">
                <p className="text-gray-500 text-lg">No results found</p>
                <p className="text-gray-400 text-sm mt-2">Try adjusting your search or filters</p>
              </div>
            )}
          </>
        ) : (
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
      </div>

      <EntityModal
        entity={selectedEntity}
        open={!!selectedEntity}
        onOpenChange={(open) => !open && setSelectedEntity(null)}
      />
    </div>
  )
}
