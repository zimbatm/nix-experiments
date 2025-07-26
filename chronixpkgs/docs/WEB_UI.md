# Chronixpkgs Web UI

The Chronixpkgs service includes a built-in web UI for visualizing and exploring GitHub events.

## Features

### 1. Recent Events Dashboard (/)
- GitHub-style event cards with icons and formatted descriptions
- Auto-refreshing event list every 10 seconds using HTMX
- Expandable raw payload viewer for each event
- Clean, modern UI using shadcn/ui design system

### 2. Live Event Stream (/stream)
- Real-time event streaming using Server-Sent Events (SSE)
- Automatic updates as new events arrive
- Visual indicators for new events
- Automatic reconnection on connection loss

## Technology Stack

- **HTMX**: For dynamic content updates without full page reloads
- **Server-Sent Events (SSE)**: For real-time event streaming
- **Go Templates**: Server-side rendering
- **shadcn/ui Design System**: Modern, accessible UI components and styling

## Usage

1. Start the Chronixpkgs services:
   ```bash
   # Terminal 1: Start the server (read-only)
   ./chronixpkgs server --listen :8081
   
   # Terminal 2: Start the fetcher (polling)
   ./chronixpkgs fetch --github-token=<token> --poll-interval 1m
   
   # Or use the development script for both:
   ./dev.sh
   ```

2. Open your browser and navigate to:
   - `http://localhost:8081/` - Main dashboard with recent events
   - `http://localhost:8081/stream` - Live event stream

## API Integration

The web UI uses the same API endpoints available for programmatic access:

- `GET /events` - Query events
- `GET /events/stream` - SSE event stream
- `GET /event-types` - List event types
- `GET /health` - Health check

All endpoints support CORS when enabled with the `--enable-cors` flag.

## Development

To modify the web UI:

1. Templates are located in `templates/`
2. Static assets (HTMX) are in `static/`
3. Server-side logic is in `internal/server/web.go`

The UI is designed to be lightweight and work without JavaScript (except for HTMX and SSE features).

## Dark Mode Support

The UI automatically adjusts to your system's dark mode preference using CSS media queries.