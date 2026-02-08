import { Entity } from '@/types/api'
import { Badge } from '@/components/ui/badge'
import { FileText, Image, Video, Music, Archive } from 'lucide-react'

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
  const Icon = getTypeIcon(entity.type)

  // Get first few attributes for preview
  const previewAttrs = entity.attributes.slice(0, 3)

  return (
    <div
      onClick={onClick}
      className="bg-white rounded-lg shadow-sm border border-gray-200 p-4 cursor-pointer hover:shadow-md transition-shadow duration-200"
    >
      <div className="flex items-start gap-3">
        <div className="flex-shrink-0 w-12 h-12 bg-gray-100 rounded-lg flex items-center justify-center">
          <Icon className="h-6 w-6 text-gray-600" />
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-start justify-between gap-2 mb-1">
            <h3 className="font-semibold text-gray-900 truncate flex-1">
              {entity.title}
            </h3>
            <Badge variant="secondary" className="flex-shrink-0 text-xs">
              {entity.type}
            </Badge>
          </div>
          {entity.description && (
            <p className="text-sm text-gray-600 line-clamp-2 mb-2">
              {entity.description}
            </p>
          )}
          <div className="flex flex-wrap gap-1">
            {previewAttrs.map((attr) => (
              <span
                key={attr.key}
                className="text-xs text-gray-500 bg-gray-100 px-2 py-0.5 rounded"
              >
                {attr.key}: {attr.value}
              </span>
            ))}
            {entity.attributes.length > 3 && (
              <span className="text-xs text-gray-400">
                +{entity.attributes.length - 3} more
              </span>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
