package web

import (
	"fmt"
)

// generateDiagramHTML generates the interactive diagram HTML page
// The JavaScript will call the API backend (s.apiURL) for data
func (s *Server) generateDiagramHTML(instanceID string) string {
	// This HTML page calls the sg_api backend via s.apiURL
	// All /api/v1/* paths are replaced with the backend URL
	apiURL := s.apiURL

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>State Guard Diagram - %s</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background: #f5f5f5;
            padding: 20px;
        }
        .container {
            max-width: 1400px;
            margin: 0 auto;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
        }
        .header {
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            color: white;
            padding: 20px 30px;
        }
        .controls {
            padding: 20px 30px;
            background: #fafafa;
            border-bottom: 1px solid #e0e0e0;
            display: flex;
            gap: 15px;
            flex-wrap: wrap;
        }
        .diagram-container {
            padding: 30px;
            min-height: 500px;
            display: flex;
            justify-content: center;
            align-items: center;
        }
        button {
            padding: 8px 16px;
            background: #667eea;
            color: white;
            border: none;
            border-radius: 4px;
            cursor: pointer;
        }
        button:hover { background: #5568d3; }
        select {
            padding: 8px 12px;
            border: 1px solid #ddd;
            border-radius: 4px;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>State Guard - FSM Diagram</h1>
            <p>Instance: <strong>%s</strong></p>
            <p style="font-size: 12px; opacity: 0.8;">API Backend: %s</p>
        </div>

        <div class="controls">
            <select id="format">
                <option value="svg" selected>SVG</option>
                <option value="png">PNG</option>
                <option value="mermaid">Mermaid</option>
                <option value="json">JSON</option>
            </select>
            <select id="layout">
                <option value="horizontal" selected>Horizontal</option>
                <option value="vertical">Vertical</option>
            </select>
            <button onclick="refreshDiagram()">Refresh</button>
            <button onclick="downloadDiagram()">Download</button>
        </div>

        <div class="diagram-container">
            <div id="diagram">Loading...</div>
        </div>
    </div>

    <script>
        const instanceID = '%s';
        const apiURL = '%s';  // Backend API server URL

        async function loadDiagram() {
            const format = document.getElementById('format').value;
            const layout = document.getElementById('layout').value;

            try {
                const url = apiURL + '/api/v1/visualize/asset/' + instanceID +
                           '?format=' + format + '&layout=' + layout + '&highlight_current=true';

                const response = await fetch(url);
                if (!response.ok) throw new Error('Failed to load diagram');

                if (format === 'svg') {
                    const svg = await response.text();
                    document.getElementById('diagram').innerHTML = svg;
                } else if (format === 'png') {
                    const blob = await response.blob();
                    const img = document.createElement('img');
                    img.src = URL.createObjectURL(blob);
                    document.getElementById('diagram').innerHTML = '';
                    document.getElementById('diagram').appendChild(img);
                } else if (format === 'json' || format === 'mermaid') {
                    const text = await response.text();
                    document.getElementById('diagram').innerHTML = '<pre>' + text + '</pre>';
                }
            } catch (error) {
                document.getElementById('diagram').innerHTML =
                    '<div style="color: red;">Error: ' + error.message + '</div>';
            }
        }

        function refreshDiagram() {
            loadDiagram();
        }

        async function downloadDiagram() {
            const format = document.getElementById('format').value;
            const layout = document.getElementById('layout').value;

            const url = apiURL + '/api/v1/visualize/asset/' + instanceID +
                       '?format=' + format + '&layout=' + layout + '&highlight_current=true';

            window.open(url, '_blank');
        }

        // Load diagram on page load and when options change
        document.addEventListener('DOMContentLoaded', loadDiagram);
        document.getElementById('format').addEventListener('change', loadDiagram);
        document.getElementById('layout').addEventListener('change', loadDiagram);
    </script>
</body>
</html>`, instanceID, instanceID, apiURL, instanceID, apiURL)
}
