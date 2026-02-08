import { Entity } from '@/types/api'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Badge } from '@/components/ui/badge'
import { FileText, Image, Video, Music, Archive, Calendar } from 'lucide-react'

interface EntityModalProps {
  entity: Entity | null
  open: boolean
  onOpenChange: (open: boolean) => void
}

function getTypeIcon(type: string) {
  const typeLower = type.toLowerCase()
  if (typeLower.includes('image') || typeLower.includes('photo')) return Image
  if (typeLower.includes('video')) return Video
  if (typeLower.includes('audio') || typeLower.includes('music')) return Music
  if (typeLower.includes('archive') || typeLower.includes('zip')) return Archive
  return FileText
}

function formatDate(timestamp: number): string {
  return new Date(timestamp * 1000).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

export function EntityModal({ entity, open, onOpenChange }: EntityModalProps) {
  if (!entity) return null

  const Icon = getTypeIcon(entity.type)

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <div className="flex items-start gap-3">
            <div className="flex-shrink-0 w-12 h-12 bg-gray-100 rounded-lg flex items-center justify-center">
              <Icon className="h-6 w-6 text-gray-600" />
            </div>
            <div className="flex-1">
              <DialogTitle className="text-xl">{entity.title}</DialogTitle>
              <div className="flex items-center gap-2 mt-2">
                <Badge variant="secondary">{entity.type}</Badge>
                {entity.provider && (
                  <Badge variant="outline" className="text-xs">
                    {entity.provider}
                  </Badge>
                )}
              </div>
            </div>
          </div>
        </DialogHeader>

        <div className="space-y-4">
          {entity.description && (
            <div>
              <h3 className="text-sm font-medium text-gray-700 mb-1">Description</h3>
              <p className="text-sm text-gray-600">{entity.description}</p>
            </div>
          )}

          <div className="flex items-center gap-2 text-sm text-gray-600">
            <Calendar className="h-4 w-4" />
            <span>Modified: {formatDate(entity.timestamp)}</span>
          </div>

          {entity.attributes.length > 0 && (
            <div>
              <h3 className="text-sm font-medium text-gray-700 mb-2">Attributes</h3>
              <dl className="grid grid-cols-2 gap-2 text-sm">
                {entity.attributes.map((attr) => (
                  <div key={attr.key} className="bg-gray-50 rounded px-3 py-2">
                    <dt className="text-xs text-gray-500">{attr.key}</dt>
                    <dd className="text-gray-900 font-medium">{attr.value}</dd>
                  </div>
                ))}
              </dl>
            </div>
          )}

          {entity.thumbnail && (
            <div>
              <h3 className="text-sm font-medium text-gray-700 mb-2">Preview</h3>
              <img
                src={entity.thumbnail}
                alt={entity.title}
                className="rounded-lg border border-gray-200 max-w-full"
              />
            </div>
          )}

          {entity.metadata && Object.keys(entity.metadata).length > 0 && (
            <div>
              <h3 className="text-sm font-medium text-gray-700 mb-2">Metadata</h3>
              <dl className="grid grid-cols-2 gap-2 text-sm">
                {Object.entries(entity.metadata).map(([key, value]) => (
                  <div key={key} className="bg-gray-50 rounded px-3 py-2">
                    <dt className="text-xs text-gray-500">{key}</dt>
                    <dd className="text-gray-900 font-medium text-xs break-all">
                      {String(value)}
                    </dd>
                  </div>
                ))}
              </dl>
            </div>
          )}

          <div className="text-xs text-gray-400 pt-2 border-t">
            ID: {entity.id}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
