import { Entity } from '@/types/api'
import { EntityCard } from './EntityCard'
import { Skeleton } from '@/components/ui/skeleton'

interface SearchResultsProps {
  entities: Entity[]
  loading: boolean
  onEntityClick: (entity: Entity) => void
}

export function SearchResults({ entities, loading, onEntityClick }: SearchResultsProps) {
  if (loading) {
    return (
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {Array.from({ length: 6 }).map((_, i) => (
          <div key={i} className="bg-white rounded-lg shadow-sm border border-gray-200 p-4">
            <Skeleton className="h-4 w-3/4 mb-2" />
            <Skeleton className="h-3 w-1/2 mb-4" />
            <Skeleton className="h-3 w-full mb-1" />
            <Skeleton className="h-3 w-2/3" />
          </div>
        ))}
      </div>
    )
  }

  if (entities.length === 0) {
    return (
      <div className="text-center py-12">
        <p className="text-gray-500 text-lg">No results found</p>
        <p className="text-gray-400 text-sm mt-2">Try adjusting your search or filters</p>
      </div>
    )
  }

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
      {entities.map((entity) => (
        <EntityCard
          key={entity.ID}
          entity={entity}
          onClick={() => onEntityClick(entity)}
        />
      ))}
    </div>
  )
}
