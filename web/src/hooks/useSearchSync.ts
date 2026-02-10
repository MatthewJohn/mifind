import { useEffect } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useSearchStore } from '@/stores/searchStore'
import type { SearchFilter } from '@/types/api'

const FILTER_PARAM_PREFIX = 'filter_'

/**
 * Hook to sync search store state with URL search params.
 * This allows refreshing the page to preserve search state and enables sharing search URLs.
 */
export function useSearchSync() {
  const [searchParams, setSearchParams] = useSearchParams()
  const {
    query,
    setQuery,
    filters,
    setFilters,
    selectedTypes,
    setSelectedTypes,
    currentPage,
    setCurrentPage,
    searchTriggered,
    triggerSearch,
  } = useSearchStore()

  // Initialize store from URL params on mount
  useEffect(() => {
    const urlQuery = searchParams.get('q') || ''
    const urlPage = parseInt(searchParams.get('page') || '1', 10)
    const urlTypes = searchParams.get('types')?.split(',').filter(Boolean) || []

    // Parse filters from URL params (format: filter_key=operator:value)
    const urlFilters: SearchFilter[] = []
    searchParams.forEach((value, key) => {
      if (key.startsWith(FILTER_PARAM_PREFIX)) {
        const filterKey = key.slice(FILTER_PARAM_PREFIX.length)
        // Format: operator:value (e.g., eq:jpg, contains:document)
        const operatorMatch = value.match(/^([^:]+):(.+)$/)
        if (operatorMatch) {
          const [, operator, filterValue] = operatorMatch
          urlFilters.push({
            key: filterKey,
            operator: operator as SearchFilter['operator'],
            value: filterValue,
          })
        } else {
          // Default to 'eq' if no operator specified
          urlFilters.push({
            key: filterKey,
            operator: 'eq',
            value,
          })
        }
      }
    })

    // Only update store if URL has params (don't override empty defaults)
    if (urlQuery || urlFilters.length > 0 || urlTypes.length > 0) {
      setQuery(urlQuery)
      setFilters(urlFilters)
      setSelectedTypes(urlTypes)
      setCurrentPage(urlPage)

      // Auto-trigger search if URL has query or filters
      if (urlQuery || urlFilters.length > 0 || urlTypes.length > 0) {
        // Small delay to ensure store is updated
        setTimeout(() => triggerSearch(), 0)
      }
    }
  }, []) // Run once on mount

  // Update URL params when store state changes
  useEffect(() => {
    // Don't update URL until user has triggered a search
    if (!searchTriggered) {
      return
    }

    const newParams = new URLSearchParams()

    // Add query
    if (query) {
      newParams.set('q', query)
    }

    // Add page
    if (currentPage > 1) {
      newParams.set('page', currentPage.toString())
    }

    // Add types
    if (selectedTypes.length > 0) {
      newParams.set('types', selectedTypes.join(','))
    }

    // Add filters (format: filter_key=operator:value)
    filters.forEach((filter) => {
      const key = `${FILTER_PARAM_PREFIX}${filter.key}`
      newParams.set(key, `${filter.operator}:${filter.value}`)
    })

    // Update URL without triggering a navigation
    setSearchParams(newParams, { replace: true })
  }, [query, filters, selectedTypes, currentPage, searchTriggered, setSearchParams])
}
