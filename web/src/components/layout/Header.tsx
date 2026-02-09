import { Search } from 'lucide-react'
import { useSearchStore } from '@/stores/searchStore'
import { Button } from '@/components/ui/button'

export function Header() {
  const { query, setQuery, triggerSearch } = useSearchStore()

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    triggerSearch()
  }

  return (
    <header className="bg-white border-b border-gray-200 sticky top-0 z-40">
      <div className="container mx-auto px-4 py-4">
        <div className="flex items-center justify-between gap-4">
          <div className="flex items-center gap-2">
            <Search className="h-6 w-6 text-[#0654ba]" />
            <h1 className="text-xl font-bold text-gray-900">mifind</h1>
          </div>
          <form onSubmit={handleSubmit} className="flex-1 max-w-2xl flex gap-2">
            <input
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search everything... (press Enter or click Search)"
              className="flex-1 px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-[#0654ba] focus:border-transparent"
            />
            <Button type="submit" size="default">
              <Search className="h-4 w-4 mr-2" />
              Search
            </Button>
          </form>
        </div>
      </div>
    </header>
  )
}
