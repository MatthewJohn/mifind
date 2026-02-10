import { useSearchStore } from '@/stores/searchStore'
import { useFilters } from '@/hooks/useSearch'
import { Filter, HardDrive, Folder, FileType } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { useState } from 'react'
import type { SearchFilter, SearchResult } from '@/types/api'

interface FilterSidebarProps {
  searchResult?: SearchResult
}

export function FilterSidebar({ searchResult }: FilterSidebarProps) {
  const {
    isFilterOpen,
    setFilterOpen,
    selectedTypes,
    toggleType,
    filters,
    addFilter,
    addFilters,
    removeFilter,
    removeFiltersByKey,
    clearFilters,
  } = useSearchStore()

  // Always fetch filters to show available types and capabilities
  // The filters endpoint returns type counts which are useful to see before searching
  // If searchResult is provided with filters/capabilities, use those instead
  const { data: filterDataFallback, isLoading } = useFilters(searchResult ? undefined : '')

  // Use searchResult filters if available, otherwise fall back to API call
  const filterData = searchResult
    ? { filters: searchResult.filters, capabilities: searchResult.capabilities, values: undefined }
    : filterDataFallback

  // Local state for filter inputs
  const [pathFilter, setPathFilter] = useState('')

  if (!isFilterOpen) {
    return (
      <button
        onClick={() => setFilterOpen(true)}
        className="fixed left-4 top-1/2 -translate-y-1/2 z-30 bg-white p-2 rounded-lg shadow-lg border border-gray-200 hover:bg-gray-50"
        title="Open filters"
      >
        <Filter className="h-5 w-5 text-gray-600" />
      </button>
    )
  }

  // Size presets in user-friendly format
  const sizePresets = [
    { label: '< 1 MB', min: 0, max: 1024 * 1024 },
    { label: '1 MB - 10 MB', min: 1024 * 1024, max: 10 * 1024 * 1024 },
    { label: '10 MB - 100 MB', min: 10 * 1024 * 1024, max: 100 * 1024 * 1024 },
    { label: '> 100 MB', min: 100 * 1024 * 1024, max: null },
  ]

  const applyPathFilter = () => {
    if (pathFilter.trim()) {
      // Remove existing path filter if any, then add new one
      removeFilter('path')
      addFilter({ key: 'path', operator: 'contains', value: pathFilter })
    }
  }

  const applyExtensionFilter = (extension: string) => {
    if (extension) {
      removeFilter('extension')
      addFilter({ key: 'extension', operator: 'eq', value: extension })
    } else {
      removeFilter('extension')
    }
  }

  const applySizePreset = (min: number | null, max: number | null) => {
    // Remove all existing size filters
    removeFiltersByKey('size')

    // Add new size filters
    const newFilters: SearchFilter[] = []
    if (min !== null) {
      newFilters.push({ key: 'size', operator: 'gte', value: min })
    }
    if (max !== null) {
      newFilters.push({ key: 'size', operator: 'lte', value: max })
    }

    if (newFilters.length > 0) {
      addFilters(newFilters)
    }
  }

  const clearSizeFilters = () => {
    removeFiltersByKey('size')
  }

  // Check if a filter is active
  const hasFilter = (key: string) => filters.some(f => f.key === key)

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
          <div className="space-y-6">
            {/* Type filters */}
            {(filterData.filters?.TypeCounts && Object.keys(filterData.filters.TypeCounts).length > 0) && (
              <div>
                <h3 className="text-sm font-medium text-gray-700 mb-2 flex items-center gap-1">
                  <FileType className="h-3 w-3" />
                  Type
                </h3>
                <div className="space-y-1">
                  {Object.entries(filterData.filters.TypeCounts)
                    .sort(([, a], [, b]) => b - a)
                    .map(([type, count]) => (
                      <label key={type} className="flex items-center gap-2 cursor-pointer group">
                        <input
                          type="checkbox"
                          checked={selectedTypes.includes(type)}
                          onChange={() => toggleType(type)}
                          className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                        />
                        <span className="text-sm text-gray-600 group-hover:text-gray-900">
                          {type} <span className="text-gray-400">({count})</span>
                        </span>
                      </label>
                    ))}
                </div>
              </div>
            )}

            {/* Path filter - always show for filesystem */}
            <div>
              <h3 className="text-sm font-medium text-gray-700 mb-2 flex items-center gap-1">
                <Folder className="h-3 w-3" />
                Path
              </h3>
              <div className="flex gap-1">
                <input
                  type="text"
                  value={pathFilter}
                  onChange={(e) => setPathFilter(e.target.value)}
                  placeholder="/home/user/Documents"
                  className="flex-1 text-sm border border-gray-300 rounded px-2 py-1.5"
                  onKeyPress={(e) => e.key === 'Enter' && applyPathFilter()}
                />
                <button
                  onClick={applyPathFilter}
                  className="px-3 py-1 text-sm bg-blue-600 text-white rounded hover:bg-blue-700"
                >
                  Apply
                </button>
              </div>
              <p className="text-xs text-gray-400 mt-1">Filter by file path (supports wildcards)</p>
            </div>

            {/* Extension filter - common ones */}
            <div>
              <h3 className="text-sm font-medium text-gray-700 mb-2 flex items-center gap-1">
                <FileType className="h-3 w-3" />
                Extension
              </h3>
              <select
                onChange={(e) => applyExtensionFilter(e.target.value)}
                className={`w-full text-sm border rounded px-2 py-1.5 ${hasFilter('extension') ? 'border-blue-500 bg-blue-50' : 'border-gray-300'}`}
              >
                <option value="">All files</option>
                <option value="pdf">PDF</option>
                <option value="docx">Word document</option>
                <option value="doc">Word document (old)</option>
                <option value="xlsx">Excel spreadsheet</option>
                <option value="xls">Excel (old)</option>
                <option value="pptx">PowerPoint</option>
                <option value="jpg">JPEG image</option>
                <option value="jpeg">JPEG image (alt)</option>
                <option value="png">PNG image</option>
                <option value="gif">GIF image</option>
                <option value="svg">SVG image</option>
                <option value="webp">WebP image</option>
                <option value="mp4">MP4 video</option>
                <option value="mov">MOV video</option>
                <option value="avi">AVI video</option>
                <option value="mkv">MKV video</option>
                <option value="mp3">MP3 audio</option>
                <option value="wav">WAV audio</option>
                <option value="flac">FLAC audio</option>
                <option value="zip">ZIP archive</option>
                <option value="tar">TAR archive</option>
                <option value="gz">GZ archive</option>
                <option value="rar">RAR archive</option>
                <option value="7z">7-Zip archive</option>
              </select>
            </div>

            {/* Size filter with presets */}
            <div>
              <h3 className="text-sm font-medium text-gray-700 mb-2 flex items-center justify-between">
                <span className="flex items-center gap-1">
                  <HardDrive className="h-3 w-3" />
                  File Size
                </span>
                {hasFilter('size') && (
                  <button
                    onClick={clearSizeFilters}
                    className="text-xs text-blue-600 hover:text-blue-800"
                  >
                    Clear
                  </button>
                )}
              </h3>
              <div className="grid grid-cols-2 gap-2">
                {sizePresets.map((preset) => {
                  const isActive = filters.some(f =>
                    f.key === 'size' &&
                    ((f.operator === 'gte' && f.value === preset.min) ||
                     (f.operator === 'lte' && f.value === preset.max))
                  )
                  return (
                    <button
                      key={preset.label}
                      onClick={() => isActive ? clearSizeFilters() : applySizePreset(preset.min, preset.max)}
                      className={`text-xs border rounded px-2 py-1 text-left ${
                        isActive
                          ? 'border-blue-500 bg-blue-50 text-blue-700'
                          : 'border-gray-300 hover:bg-gray-50'
                      }`}
                    >
                      {preset.label}
                    </button>
                  )
                })}
              </div>
            </div>

            {/* Show more capabilities if available */}
            {filterData.capabilities && Object.keys(filterData.capabilities).length > 4 && (
              <details className="group" open={!!filterData.values && Object.keys(filterData.values).length > 0}>
                <summary className="text-sm font-medium text-gray-700 cursor-pointer hover:text-gray-900">
                  More filters ▼
                </summary>
                <div className="mt-3 space-y-3 pl-2 border-l-2 border-gray-200">
                  {Object.entries(filterData.capabilities)
                    .filter(([key]) => !['type', 'path', 'extension', 'size'].includes(key))
                    .slice(0, 10) // Show up to 10 additional filters
                    .map(([key, cap]: [string, any]) => {
                      // Check if we have pre-obtained values from the API
                      const preObtainedValues = filterData.values?.[key]
                      // Fall back to options from capabilities (for providers that include them)
                      const options = preObtainedValues || cap.Options
                      const hasOptions = options && options.length > 0

                      // Don't show filters with no options and no description (likely not useful)
                      if (!hasOptions && !cap.Description) {
                        return null
                      }

                      // Person filter gets special multi-select treatment
                      const isPersonFilter = key === 'person'
                      const currentFilterValues = filters
                        .filter(f => f.key === key)
                        .map(f => String(f.value))

                      return (
                        <div key={key}>
                          <div className="flex items-center justify-between mb-1">
                            <h4 className="text-xs font-medium text-gray-500">{cap.Description || key}</h4>
                            {currentFilterValues.length > 0 && (
                              <button
                                onClick={() => removeFiltersByKey(key)}
                                className="text-xs text-blue-600 hover:text-blue-800"
                              >
                                Clear
                              </button>
                            )}
                          </div>
                          {hasOptions ? (
                            isPersonFilter ? (
                              // Multi-select for people using checkboxes
                              <div className="max-h-40 overflow-y-auto border border-gray-200 rounded p-2 space-y-1">
                                {options.map((opt: any) => (
                                  <label key={opt.value} className="flex items-center gap-2 cursor-pointer group text-xs">
                                    <input
                                      type="checkbox"
                                      checked={currentFilterValues.includes(opt.value)}
                                      onChange={(e) => {
                                        if (e.target.checked) {
                                          addFilter({ key, operator: 'eq', value: opt.value })
                                        } else {
                                          // Remove this specific value
                                          const filterToRemove = filters.find(f => f.key === key && String(f.value) === opt.value)
                                          if (filterToRemove) {
                                            // Note: we'd need to enhance removeFilter to handle by key+value
                                            // For now, just remove all and re-add the others
                                            removeFiltersByKey(key)
                                            currentFilterValues
                                              .filter(v => v !== opt.value)
                                              .forEach(v => addFilter({ key, operator: 'eq', value: v }))
                                          }
                                        }
                                      }}
                                      className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                                    />
                                    <span className="text-gray-600 group-hover:text-gray-900">
                                      {opt.label || opt.value}
                                      {opt.count > 0 && (
                                        <span className="text-gray-400">
                                          {' '}({opt.count}{opt.has_more ? '+' : ''})
                                        </span>
                                      )}
                                    </span>
                                  </label>
                                ))}
                              </div>
                            ) : (
                              // Single select for other filters
                              <select
                                className="w-full text-xs border border-gray-300 rounded px-2 py-1"
                                onChange={(e) => {
                                  const value = e.target.value
                                  if (value) {
                                    removeFiltersByKey(key)
                                    addFilter({ key, operator: 'eq', value })
                                  } else {
                                    removeFiltersByKey(key)
                                  }
                                }}
                                value={currentFilterValues[0] ?? ''}
                              >
                                <option value="">All</option>
                                {options.map((opt: any) => (
                                  <option key={opt.value} value={opt.value}>
                                    {opt.label || opt.value}
                                    {opt.count > 0 ? ` (${opt.count}${opt.has_more ? '+' : ''})` : ''}
                                  </option>
                                ))}
                              </select>
                            )
                          ) : (
                            <input
                              type="text"
                              placeholder={`Filter by ${key}`}
                              className="w-full text-xs border border-gray-300 rounded px-2 py-1"
                              onKeyPress={(e) => {
                                if (e.key === 'Enter') {
                                  const target = e.target as HTMLInputElement
                                  if (target.value.trim()) {
                                    removeFiltersByKey(key)
                                    addFilter({ key, operator: 'contains', value: target.value.trim() })
                                  }
                                }
                              }}
                            />
                          )}
                        </div>
                      )
                    })}
                </div>
              </details>
            )}
          </div>
        )}

        {isLoading && (
          <div className="text-sm text-gray-500">Loading filters...</div>
        )}
      </div>
    </div>
  )
}
