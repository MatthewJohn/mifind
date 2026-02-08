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
    query,
  } = useSearchStore()

  const { data: filterData, isLoading } = useFilters(query)

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
            {/* Type filter - use capabilities.type.Options if available, otherwise TypeCounts from search results */}
            {(filterData.capabilities?.['type']?.Options || (filterData.filters?.TypeCounts && Object.keys(filterData.filters.TypeCounts).length > 0)) && (
              <div className="mb-6">
                <h3 className="text-sm font-medium text-gray-700 mb-2">Type</h3>
                <div className="space-y-1">
                  {filterData.filters?.TypeCounts ? (
                    // Show types with counts from search results
                    Object.entries(filterData.filters.TypeCounts).map(([type, count]) => (
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
                    ))
                  ) : (
                    // Show predefined types from capabilities
                    filterData.capabilities?.['type']?.Options?.map((opt: any) => (
                      <label key={opt.Value} className="flex items-center gap-2 cursor-pointer">
                        <input
                          type="checkbox"
                          checked={selectedTypes.includes(opt.Value)}
                          onChange={() => toggleType(opt.Value)}
                          className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                        />
                        <span className="text-sm text-gray-600">
                          {opt.Label || opt.Value}
                        </span>
                      </label>
                    ))
                  )}
                </div>
              </div>
            )}

            {/* Attribute filters - show from capabilities */}
            {filterData.capabilities && (
              <div>
                <h3 className="text-sm font-medium text-gray-700 mb-2">Attributes</h3>
                <div className="space-y-3">
                  {Object.entries(filterData.capabilities)
                    .filter(([key]) => key !== 'type') // Skip type, already shown above
                    .slice(0, 8)
                    .map(([key, cap]: [string, any]) => (
                      <div key={key}>
                        <h4 className="text-xs font-medium text-gray-500 mb-1">{cap.Description || key}</h4>
                        {cap.Options ? (
                          // Predefined options
                          <select className="w-full text-sm border border-gray-300 rounded px-2 py-1">
                            <option value="">All</option>
                            {cap.Options.map((opt: any) => (
                              <option key={opt.Value} value={opt.Value}>
                                {opt.Label || opt.Value} {opt.Count > 0 && `(${opt.Count})`}
                              </option>
                            ))}
                          </select>
                        ) : cap.Type === 'string' || cap.Type === 'time' ? (
                          // Text input for strings and time
                          <input
                            type="text"
                            placeholder={`Filter by ${key}`}
                            className="w-full text-sm border border-gray-300 rounded px-2 py-1"
                          />
                        ) : cap.Type === 'int64' || cap.Type === 'int' ? (
                          // Number input
                          <input
                            type="number"
                            placeholder={`Filter by ${key}`}
                            className="w-full text-sm border border-gray-300 rounded px-2 py-1"
                          />
                        ) : (
                          // Default text input
                          <input
                            type="text"
                            placeholder={`Filter by ${key}`}
                            className="w-full text-sm border border-gray-300 rounded px-2 py-1"
                          />
                        )}
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
