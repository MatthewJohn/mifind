# mifind Web UI

A modern, eBay-style React web interface for mifind universal search.

## Tech Stack

- **Vite** - Fast build tool with HMR
- **React 18 + TypeScript** - UI framework
- **Tailwind CSS** - Styling
- **shadcn/ui** - High-quality React components
- **React Query (TanStack Query)** - API state management
- **Zustand** - Lightweight UI state management
- **Lucide React** - Icon library

## Development

### Prerequisites

- Node.js 18+ and npm
- Go 1.21+ (for backend)
- mifind API server running on port 8080

### Install Dependencies

```bash
cd web
npm install
```

### Development Mode

Run Vite dev server (terminal 1):
```bash
npm run dev
```

Run mifind API (terminal 2):
```bash
cd ..
go run cmd/mifind/main.go
```

Visit http://localhost:5173

### Build for Production

The React build needs to be copied to the Go embed directory before building the Go binary:

```bash
cd web
./copy-to-api.sh
cd ..
go build -o mifind cmd/mifind/main.go
./mifind
```

Visit http://localhost:8080

## Architecture

### Components

- `components/ui/` - shadcn/ui base components
- `components/layout/` - Layout components (MainLayout, Header)
- `components/search/` - Search components (SearchBar, EntityCard, SearchResults, FilterSidebar)
- `components/entity/` - Entity detail components (EntityModal)

### State Management

- **Zustand** (`stores/searchStore.ts`) - UI state (query, filters, selected entity)
- **React Query** (`hooks/useSearch.ts`) - API state (search results, filters, providers)

### API Integration

The UI connects to the mifind API at `/api` endpoints:
- `POST /api/search` - Search
- `GET /api/entity/{id}` - Get entity details
- `GET /api/filters` - Get available filters
- `GET /api/providers` - List providers

## Design

The UI follows an eBay-inspired design:
- Light gray background (#f5f5f5)
- White cards with subtle shadows
- Blue accent (#0654ba)
- Responsive grid (1-4 columns)
- Clean typography with clear hierarchy
