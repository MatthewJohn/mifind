import { Entity } from '@/types/api'
import { Badge } from '@/components/ui/badge'
import { FileText, Image, Video, Music, Archive, ExternalLink } from 'lucide-react'
import { useSearchStore } from '@/stores/searchStore'

interface EntityCardProps {
  entity: Entity
  onClick: () => void
}

function getTypeIcon(type: string) {
  const typeLower = type.toLowerCase()
  if (typeLower.includes('image') || typeLower.includes('photo')) return Image
  if (typeLower.includes('video')) return Video
  if (typeLower.includes('audio') || typeLower.includes('music')) return Music
  if (typeLower.includes('archive') || typeLower.includes('zip')) return Archive
  return FileText
}

export function EntityCard({ entity, onClick }: EntityCardProps) {
  const Icon = getTypeIcon(entity.Type)
  const { addFilter } = useSearchStore()

  // Get first few attributes for preview (excluding web_url and thumbnail_url)
  const attributeEntries = Object.entries(entity.Attributes || {})
    .filter(([key]) => !['web_url', 'thumbnail_url'].includes(key))
    .slice(0, 3)

  // Get thumbnail URL
  const thumbnailUrl = entity.Thumbnail || (entity.Attributes?.thumbnail_url as string)

  // Get web URL
  const webUrl = entity.Attributes?.web_url as string

  // Quick add filter function
  const quickAddFilter = (key: string, value: any) => {
    addFilter({ key, operator: 'eq', value: String(value) })
  }

  // Handle card click - for person entities, add filter instead of opening details
  const handleCardClick = () => {
    if (entity.Type === 'person' && entity.Provider === 'immich') {
      // For person entities, add them as a filter instead of opening details
      // Extract the person ID from the entity ID (format: immich:instanceID:personID)
      const parts = entity.ID.split(':')
      if (parts.length >= 3) {
        const personID = parts[2]
        addFilter({ key: 'person', operator: 'eq', value: personID })
      }
    } else {
      onClick()
    }
  }

  return (
    <div
      onClick={handleCardClick}
      className="bg-white rounded-lg shadow-sm border border-gray-200 p-4 cursor-pointer hover:shadow-md transition-shadow duration-200"
    >
      <div className="flex items-start gap-3">
        {/* Thumbnail or icon */}
        {thumbnailUrl ? (
          <div className="flex-shrink-0 w-16 h-16 bg-gray-100 rounded-lg overflow-hidden relative">
            <img
              src={thumbnailUrl}
              alt={entity.Title}
              className="w-full h-full object-cover"
              onError={(e) => {
                // Fallback to icon on error
                e.currentTarget.style.display = 'none'
                const parent = e.currentTarget.parentElement
                if (parent) {
                  const iconContainer = document.createElement('div')
                  iconContainer.className = 'absolute inset-0 flex items-center justify-center'
                  const icon = document.createElement('div')
                  icon.innerHTML = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="text-gray-600"><path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"/><polyline points="14 2 14 8 20 8"/></svg>`
                  iconContainer.appendChild(icon)
                  parent.appendChild(iconContainer)
                }
              }}
            />
          </div>
        ) : (
          <div className="flex-shrink-0 w-12 h-12 bg-gray-100 rounded-lg flex items-center justify-center">
            <Icon className="h-6 w-6 text-gray-600" />
          </div>
        )}

        <div className="flex-1 min-w-0">
          <div className="flex items-start justify-between gap-2 mb-1">
            <h3 className="font-semibold text-gray-900 truncate flex-1">
              {entity.Title}
            </h3>
            <div className="flex items-center gap-1 flex-shrink-0">
              <Badge variant="secondary" className="text-xs">
                {entity.Type}
              </Badge>
              {webUrl && (
                <a
                  href={webUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="p-1 bg-gray-100 rounded hover:bg-gray-200"
                  title="Open in Immich"
                  onClick={(e) => e.stopPropagation()}
                >
                  <ExternalLink className="h-3 w-3 text-gray-600" />
                </a>
              )}
            </div>
          </div>
          {entity.Description && (
            <p className="text-sm text-gray-600 line-clamp-2 mb-2">
              {entity.Description}
            </p>
          )}
          <div className="flex flex-wrap gap-1">
            {attributeEntries.map(([key, value]) => (
              <span
                key={key}
                className="text-xs text-gray-500 bg-gray-100 px-2 py-0.5 rounded"
              >
                {key}: {String(value)}
              </span>
            ))}
            {Object.keys(entity.Attributes || {}).length > 3 && (
              <span className="text-xs text-gray-400">
                +{Object.keys(entity.Attributes || {}).length - 3} more
              </span>
            )}
          </div>

          {/* Quick filter buttons for Immich entities */}
          {entity.Provider === 'immich' && entity.Type !== 'person' && (
            <div className="flex flex-wrap gap-2 mt-3">
              {entity.Attributes?.person && (
                <button
                  onClick={(e) => {
                    e.stopPropagation()
                    quickAddFilter('person', entity.Attributes?.person)
                  }}
                  className="text-xs px-2 py-1 bg-blue-100 text-blue-700 rounded hover:bg-blue-200 transition-colors"
                >
                  + Person
                </button>
              )}
              {entity.Attributes?.album && (
                <button
                  onClick={(e) => {
                    e.stopPropagation()
                    quickAddFilter('album', entity.Attributes?.album)
                  }}
                  className="text-xs px-2 py-1 bg-purple-100 text-purple-700 rounded hover:bg-purple-200 transition-colors"
                >
                  + Album
                </button>
              )}
              {entity.Attributes?.location && (
                <button
                  onClick={(e) => {
                    e.stopPropagation()
                    quickAddFilter('location', entity.Attributes?.location)
                  }}
                  className="text-xs px-2 py-1 bg-green-100 text-green-700 rounded hover:bg-green-200 transition-colors"
                >
                  + Location
                </button>
              )}
            </div>
          )}

          {/* For person entities, show a hint that clicking will filter */}
          {entity.Type === 'person' && entity.Provider === 'immich' && (
            <div className="mt-2 text-xs text-blue-600">
              Click to filter photos by this person
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
