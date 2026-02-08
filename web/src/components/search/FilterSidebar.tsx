import { useSearchStore } from '@/stores/searchStore'
import { useFilters } from '@/hooks/useSearch'
import { Filter } from 'lucide-react'
import { Badge } from '@/components/ui/badge'

export function FilterSidebar() {
  const {
    isFilterOpen,
    setFilterOpen,
    selectedTypes,
    toggleType,
    filters,
    removeFilter,
    clearFilters,
  } = useSearchStore()

  const { data: filterData, isLoading } = useFilters()

  if (!isFilterOpen) {
    return (
      <button
        onClick={() => setFilterOpen(true)}
        className="fixed left-4 top-1/2 -translate-y-1/2 z-30 bg-white p-2 rounded-lg shadow-lg border border-gray-200 hover:bg-gray-50"
      >
        <Filter className="h-5 w-5 text-gray-600" />
      </button>
    )
  }

  return (
    <div className="fixed left-0 top-0 h-full w-80 bg-white border-r border-gray-200 overflow-y-auto z-30 shadow-lg">
      <div className="p-4">
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-lg font-semibold">Filters</h2>
          <button
            onClick={() => setFilterOpen(false)}
            className="text-gray-500 hover:text-gray-700"
          >
            ✕
          </button>
        </div>

        {filters.length > 0 && (
          <div className="mb-6">
            <div className="flex items-center justify-between mb-2">
              <h3 className="text-sm font-medium text-gray-700">Active Filters</h3>
              <button
                onClick={clearFilters}
                className="text-xs text-blue-600 hover:text-blue-800"
              >
                Clear all
              </button>
            </div>
            <div className="flex flex-wrap gap-2">
              {filters.map((filter) => (
                <Badge key={filter.key} variant="secondary" className="text-xs">
                  {filter.key}: {String(filter.value)}
                  <button
                    onClick={() => removeFilter(filter.key)}
                    className="ml-1 hover:text-red-600"
                  >
                    ×
                  </button>
                </Badge>
              ))}
            </div>
          </div>
        )}

        {!isLoading && filterData && (
          <>
            {filterData.filters && filterData.filters.TypeCounts && Object.keys(filterData.filters.TypeCounts).length > 0 && (
              <div className="mb-6">
                <h3 className="text-sm font-medium text-gray-700 mb-2">Type</h3>
                <div className="space-y-1">
                  {Object.entries(filterData.filters.TypeCounts).map(([type, count]) => (
                    <label key={type} className="flex items-center gap-2 cursor-pointer">
                      <input
                        type="checkbox"
                        checked={selectedTypes.includes(type)}
                        onChange={() => toggleType(type)}
                        className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                      />
                      <span className="text-sm text-gray-600">
                        {type} ({count})
                      </span>
                    </label>
                  ))}
                </div>
              </div>
            )}

            {filterData.filters && filterData.filters.AttributeCounts && (
              <div>
                <h3 className="text-sm font-medium text-gray-700 mb-2">Attributes</h3>
                <div className="space-y-3">
                  {Object.entries(filterData.filters.AttributeCounts).slice(0, 5).map(([attr, values]) => (
                    <div key={attr}>
                      <h4 className="text-xs font-medium text-gray-500 mb-1">{attr}</h4>
                      <select className="w-full text-sm border border-gray-300 rounded px-2 py-1">
                        <option value="">All</option>
                        {Object.entries(values as Record<string, number>).map(([value, count]) => (
                          <option key={value} value={value}>
                            {value} ({count})
                          </option>
                        ))}
                      </select>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </>
        )}

        {isLoading && (
          <div className="text-sm text-gray-500">Loading filters...</div>
        )}
      </div>
    </div>
  )
}
