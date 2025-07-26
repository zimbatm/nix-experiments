package server

import (
	"html/template"
	"net/http"
	"time"
)

const rootHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Chronixpkgs - GitHub Event Chronicle</title>
    <script src="https://unpkg.com/htmx.org@1.9.10"></script>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
            background: #f5f5f5;
        }
        .header {
            background: white;
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 20px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        h1 {
            margin: 0 0 10px 0;
            color: #333;
        }
        .subtitle {
            color: #666;
            margin: 0;
        }
        .container {
            display: grid;
            grid-template-columns: 300px 1fr;
            gap: 20px;
        }
        .sidebar {
            background: white;
            padding: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            height: fit-content;
        }
        .main {
            background: white;
            padding: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .endpoint {
            margin-bottom: 20px;
            padding: 15px;
            background: #f8f9fa;
            border-radius: 4px;
            border: 1px solid #e9ecef;
        }
        .endpoint h3 {
            margin: 0 0 10px 0;
            color: #495057;
        }
        .endpoint code {
            background: #e9ecef;
            padding: 2px 6px;
            border-radius: 3px;
            font-size: 14px;
        }
        button {
            background: #007bff;
            color: white;
            border: none;
            padding: 8px 16px;
            border-radius: 4px;
            cursor: pointer;
            font-size: 14px;
        }
        button:hover {
            background: #0056b3;
        }
        .event {
            padding: 10px;
            border-bottom: 1px solid #e9ecef;
        }
        .event:last-child {
            border-bottom: none;
        }
        .event-type {
            display: inline-block;
            padding: 2px 8px;
            background: #e9ecef;
            border-radius: 3px;
            font-size: 12px;
            margin-right: 10px;
        }
        .loading {
            color: #666;
            font-style: italic;
        }
        #event-types {
            margin-top: 10px;
        }
        .type-filter {
            margin: 5px 0;
        }
        .type-filter label {
            display: block;
            padding: 5px;
            cursor: pointer;
        }
        .type-filter label:hover {
            background: #f8f9fa;
        }
        .health-status {
            display: inline-block;
            padding: 4px 12px;
            border-radius: 20px;
            font-size: 14px;
            font-weight: 500;
        }
        .health-ok {
            background: #d4edda;
            color: #155724;
        }
        .health-degraded {
            background: #fff3cd;
            color: #856404;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>Chronixpkgs</h1>
        <p class="subtitle">GitHub Event Chronicle for {{.Repository}}</p>
        <p>Health: <span id="health-status" hx-get="/health" hx-trigger="load, every 30s" hx-target="this"></span></p>
    </div>
    
    <div class="container">
        <div class="sidebar">
            <h2>Event Types</h2>
            <button hx-get="/event-types" 
                    hx-target="#event-types" 
                    hx-indicator="#types-loading">
                Load Event Types
            </button>
            <span id="types-loading" class="loading htmx-indicator">Loading...</span>
            <div id="event-types"></div>
        </div>
        
        <div class="main">
            <h2>API Endpoints</h2>
            
            <div class="endpoint">
                <h3>Get Events (REST API & SSE)</h3>
                <p>Retrieve or stream events for the repository:</p>
                <code>GET /events</code>
                <p>Parameters: <code>since</code>, <code>type</code>, <code>actor</code>, <code>pr</code>, <code>issue</code>, <code>limit</code>, <code>offset</code></p>
                <p>For SSE streaming, add header: <code>Accept: text/event-stream</code></p>
                <button hx-get="/events?limit=10" 
                        hx-target="#recent-events" 
                        hx-indicator="#events-loading">
                    Load Recent Events
                </button>
                <span id="events-loading" class="loading htmx-indicator">Loading...</span>
                <div id="recent-events"></div>
            </div>
            
            <div class="endpoint">
                <h3>Real-time Event Stream</h3>
                <p>Stream events as they arrive using Server-Sent Events:</p>
                <button onclick="startEventStream()">Start Live Stream</button>
                <button onclick="stopEventStream()" style="background: #dc3545;">Stop Stream</button>
                <div id="live-events" style="max-height: 300px; overflow-y: auto; margin-top: 10px;"></div>
            </div>
            
        </div>
    </div>

    <script>
        let eventSource = null;
        
        function startEventStream() {
            if (eventSource) return;
            
            const container = document.getElementById('live-events');
            container.innerHTML = '<p class="loading">Connecting to event stream...</p>';
            
            eventSource = new EventSource('/events');
            
            eventSource.onmessage = function(e) {
                if (e.event === 'ping') return;
                
                const event = JSON.parse(e.data);
                const eventEl = document.createElement('div');
                eventEl.className = 'event';
                eventEl.innerHTML = ` + "`" + `
                    <span class="event-type">${event.event_type}</span>
                    <strong>${event.actor}</strong>
                    <small>${new Date(event.created_at).toLocaleString()}</small>
                ` + "`" + `;
                
                if (container.firstChild && container.firstChild.className === 'loading') {
                    container.innerHTML = '';
                }
                
                container.insertBefore(eventEl, container.firstChild);
                
                // Keep only last 20 events
                while (container.children.length > 20) {
                    container.removeChild(container.lastChild);
                }
            };
            
            eventSource.onerror = function(e) {
                container.innerHTML = '<p style="color: red;">Connection lost. Please refresh the page.</p>';
                eventSource.close();
                eventSource = null;
            };
        }
        
        function stopEventStream() {
            if (eventSource) {
                eventSource.close();
                eventSource = null;
                document.getElementById('live-events').innerHTML = '<p>Stream stopped.</p>';
            }
        }
        
        // Custom HTMX handlers for better display
        document.body.addEventListener('htmx:afterSwap', function(evt) {
            if (evt.detail.target.id === 'health-status') {
                const data = JSON.parse(evt.detail.xhr.responseText);
                const statusEl = evt.detail.target;
                statusEl.className = 'health-status health-' + data.status;
                statusEl.textContent = data.status.toUpperCase();
            } else if (evt.detail.target.id === 'event-types') {
                const types = JSON.parse(evt.detail.xhr.responseText);
                evt.detail.target.innerHTML = types.map(type => ` + "`" + `
                    <div class="type-filter">
                        <label>
                            <input type="checkbox" value="${type}" onchange="updateTypeFilter()">
                            ${type}
                        </label>
                    </div>
                ` + "`" + `).join('');
            } else if (evt.detail.target.id === 'recent-events') {
                const events = JSON.parse(evt.detail.xhr.responseText);
                evt.detail.target.innerHTML = '<h4>Recent Events:</h4>' + 
                    events.map(event => ` + "`" + `
                        <div class="event">
                            <span class="event-type">${event.event_type}</span>
                            <strong>${event.actor}</strong>
                            <small>${new Date(event.created_at).toLocaleString()}</small>
                        </div>
                    ` + "`" + `).join('');
            }
        });
        
        function updateTypeFilter() {
            // This would update the event filters - left as an exercise
            console.log('Filter updated');
        }
    </script>
</body>
</html>`

func (s *Server) handleRootHTML(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.New("root").Parse(rootHTMLTemplate)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	data := struct {
		Repository string
		Time       string
	}{
		Repository: s.getMonitoredRepo(),
		Time:       time.Now().Format(time.RFC3339),
	}

	if err := tmpl.Execute(w, data); err != nil {
		s.logger.WithError(err).Error("Failed to render HTML template")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
