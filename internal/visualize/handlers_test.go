package visualize

import (
	"net/http"
	"testing"
)

func TestParseOptions(t *testing.T) {
	handler := &Handler{}

	tests := []struct {
		name                    string
		queryParams             map[string]string
		expectedHighlightCurrent bool
		expectedShowAvailable   bool
		expectedFormat          Format
		expectedLayout          Layout
	}{
		{
			name:                    "default options - HighlightCurrent should be true",
			queryParams:             map[string]string{},
			expectedHighlightCurrent: true,
			expectedShowAvailable:   false,
			expectedFormat:          FormatSVG,
			expectedLayout:          LayoutHorizontal,
		},
		{
			name:                    "available=true should keep HighlightCurrent true (bug fix)",
			queryParams:             map[string]string{"available": "true"},
			expectedHighlightCurrent: true,
			expectedShowAvailable:   true,
			expectedFormat:          FormatSVG,
			expectedLayout:          LayoutHorizontal,
		},
		{
			name:                    "highlight_state parameter sets HighlightCurrent and CurrentState",
			queryParams:             map[string]string{"highlight_state": "RUNNING"},
			expectedHighlightCurrent: true,
			expectedShowAvailable:   false,
			expectedFormat:          FormatSVG,
			expectedLayout:          LayoutHorizontal,
		},
		{
			name:                    "both available and highlight_state work together",
			queryParams:             map[string]string{"available": "true", "highlight_state": "RUNNING"},
			expectedHighlightCurrent: true,
			expectedShowAvailable:   true,
			expectedFormat:          FormatSVG,
			expectedLayout:          LayoutHorizontal,
		},
		{
			name:                    "custom format and layout",
			queryParams:             map[string]string{"format": "png", "layout": "vertical"},
			expectedHighlightCurrent: true,
			expectedShowAvailable:   false,
			expectedFormat:          FormatPNG,
			expectedLayout:          LayoutVertical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request with query parameters
			req, err := http.NewRequest("GET", "http://example.com/test", nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			q := req.URL.Query()
			for key, value := range tt.queryParams {
				q.Set(key, value)
			}
			req.URL.RawQuery = q.Encode()

			// Parse options
			format, layout, opts, err := handler.parseOptions(req)
			if err != nil {
				t.Fatalf("parseOptions() error = %v", err)
			}

			// Verify HighlightCurrent
			if opts.HighlightCurrent != tt.expectedHighlightCurrent {
				t.Errorf("HighlightCurrent = %v, expected %v", opts.HighlightCurrent, tt.expectedHighlightCurrent)
			}

			// Verify ShowAvailable
			if opts.ShowAvailable != tt.expectedShowAvailable {
				t.Errorf("ShowAvailable = %v, expected %v", opts.ShowAvailable, tt.expectedShowAvailable)
			}

			// Verify format
			if format != tt.expectedFormat {
				t.Errorf("Format = %v, expected %v", format, tt.expectedFormat)
			}

			// Verify layout
			if layout != tt.expectedLayout {
				t.Errorf("Layout = %v, expected %v", layout, tt.expectedLayout)
			}

			// Additional check for highlight_state parameter
			if highlightState, ok := tt.queryParams["highlight_state"]; ok {
				if opts.CurrentState != highlightState {
					t.Errorf("CurrentState = %v, expected %v", opts.CurrentState, highlightState)
				}
			}
		})
	}
}

func TestParseOptionsColorScheme(t *testing.T) {
	handler := &Handler{}

	tests := []struct {
		name           string
		schemeParam    string
		expectedScheme string
	}{
		{
			name:           "default color scheme is light",
			schemeParam:    "",
			expectedScheme: "light",
		},
		{
			name:           "dark color scheme",
			schemeParam:    "dark",
			expectedScheme: "dark",
		},
		{
			name:           "light color scheme explicit",
			schemeParam:    "light",
			expectedScheme: "light",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", "http://example.com/test", nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			if tt.schemeParam != "" {
				q := req.URL.Query()
				q.Set("scheme", tt.schemeParam)
				req.URL.RawQuery = q.Encode()
			}

			_, _, opts, err := handler.parseOptions(req)
			if err != nil {
				t.Fatalf("parseOptions() error = %v", err)
			}

			if opts.ColorScheme != tt.expectedScheme {
				t.Errorf("ColorScheme = %v, expected %v", opts.ColorScheme, tt.expectedScheme)
			}
		})
	}
}

func TestParseOptionsBooleanParams(t *testing.T) {
	handler := &Handler{}

	req, err := http.NewRequest("GET", "http://example.com/test?history=true&available=true", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	_, _, opts, err := handler.parseOptions(req)
	if err != nil {
		t.Fatalf("parseOptions() error = %v", err)
	}

	if !opts.ShowHistory {
		t.Error("ShowHistory should be true when history=true")
	}

	if !opts.ShowAvailable {
		t.Error("ShowAvailable should be true when available=true")
	}

	// Ensure HighlightCurrent is still true (the bug fix)
	if !opts.HighlightCurrent {
		t.Error("HighlightCurrent should be true by default, even with other parameters set")
	}
}
