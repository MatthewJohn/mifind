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

function formatDate(timestamp: string): string {
  return new Date(timestamp).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

export function EntityModal({ entity, open, onOpenChange }: EntityModalProps) {
  if (!entity) return null

  const Icon = getTypeIcon(entity.Type)

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <div className="flex items-start gap-3">
            <div className="flex-shrink-0 w-12 h-12 bg-gray-100 rounded-lg flex items-center justify-center">
              <Icon className="h-6 w-6 text-gray-600" />
            </div>
            <div className="flex-1">
              <DialogTitle className="text-xl">{entity.Title}</DialogTitle>
              <div className="flex items-center gap-2 mt-2">
                <Badge variant="secondary">{entity.Type}</Badge>
                {entity.Provider && (
                  <Badge variant="outline" className="text-xs">
                    {entity.Provider}
                  </Badge>
                )}
              </div>
            </div>
          </div>
        </DialogHeader>

        <div className="space-y-4">
          {entity.Description && (
            <div>
              <h3 className="text-sm font-medium text-gray-700 mb-1">Description</h3>
              <p className="text-sm text-gray-600">{entity.Description}</p>
            </div>
          )}

          <div className="flex items-center gap-2 text-sm text-gray-600">
            <Calendar className="h-4 w-4" />
            <span>Modified: {formatDate(entity.Timestamp)}</span>
          </div>

          {entity.Attributes && Object.keys(entity.Attributes).length > 0 && (
            <div>
              <h3 className="text-sm font-medium text-gray-700 mb-2">Attributes</h3>
              <dl className="grid grid-cols-2 gap-2 text-sm">
                {Object.entries(entity.Attributes).map(([key, value]) => (
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

          {entity.Relationships && entity.Relationships.length > 0 && (
            <div>
              <h3 className="text-sm font-medium text-gray-700 mb-2">Relationships</h3>
              <dl className="grid grid-cols-2 gap-2 text-sm">
                {entity.Relationships.map((rel, idx) => (
                  <div key={idx} className="bg-gray-50 rounded px-3 py-2">
                    <dt className="text-xs text-gray-500">{rel.Type}</dt>
                    <dd className="text-gray-900 font-medium text-xs break-all">
                      {rel.TargetID}
                    </dd>
                  </div>
                ))}
              </dl>
            </div>
          )}

          {entity.SearchTokens && entity.SearchTokens.length > 0 && (
            <div>
              <h3 className="text-sm font-medium text-gray-700 mb-2">Search Terms</h3>
              <div className="flex flex-wrap gap-1">
                {entity.SearchTokens.map((token, idx) => (
                  <span key={idx} className="text-xs bg-gray-100 px-2 py-1 rounded">
                    {token}
                  </span>
                ))}
              </div>
            </div>
          )}

          <div className="text-xs text-gray-400 pt-2 border-t">
            ID: {entity.ID}
            {entity.score !== undefined && (
              <span className="ml-3">Score: {entity.score.toFixed(3)}</span>
            )}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
