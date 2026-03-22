package confluence

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestServer creates an httptest.Server that routes requests to handler
// functions keyed by path prefix. Unmatched paths return 404.
func newTestServer(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for prefix, h := range handlers {
			if strings.HasPrefix(r.URL.Path, prefix) {
				h(w, r)
				return
			}
		}
		http.NotFound(w, r)
	}))
}

// newTestClient creates a Client pointing at the given test server URL with
// a fresh cache and a no-op logger. It constructs the client directly to
// bypass SSRF validation (test servers bind to localhost).
func newTestClient(baseURL string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		email:      "user@example.com",
		apiToken:   "test-token",
		httpClient: &http.Client{Timeout: 15 * time.Second},
		cache:      newCache(),
		logger:     slog.Default(),
	}
}

// jsonResponse writes a JSON-encoded value with HTTP 200.
func jsonResponse(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("encoding JSON response: %v", err)
	}
}

// --- Health ---

func TestHealth(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{
			name:       "returns nil on 200",
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "returns error on 401 unauthorized",
			statusCode: http.StatusUnauthorized,
			wantErr:    true,
		},
		{
			name:       "returns error on 500 server error",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
		{
			name:       "returns error on 503 service unavailable",
			statusCode: http.StatusServiceUnavailable,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServer(t, map[string]http.HandlerFunc{
				"/rest/api/settings/lookandfeel": func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(tt.statusCode)
					_, _ = w.Write([]byte(`{}`))
				},
			})
			defer srv.Close()

			client := newTestClient(srv.URL)
			err := client.Health(context.Background())

			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// --- Authentication ---

func TestDoGet_SendsBasicAuthHeader(t *testing.T) {
	email := "admin@example.com"
	token := "super-secret-token"
	expectedCreds := base64.StdEncoding.EncodeToString([]byte(email + ":" + token))

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := &Client{
		baseURL:    strings.TrimRight(srv.URL, "/"),
		email:      email,
		apiToken:   token,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		cache:      newCache(),
		logger:     slog.Default(),
	}
	_ = client.Health(context.Background())

	want := "Basic " + expectedCreds
	if gotAuth != want {
		t.Errorf("Authorization header = %q, want %q", gotAuth, want)
	}
}

func TestDoGet_SetsAcceptJSON(t *testing.T) {
	var gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	_ = client.Health(context.Background())

	if gotAccept != "application/json" {
		t.Errorf("Accept header = %q, want %q", gotAccept, "application/json")
	}
}

// --- ListSpaces ---

func TestListSpaces(t *testing.T) {
	spacesJSON := apiSpaceResponse{
		Results: []apiSpace{
			{
				ID:     1,
				Key:    "ENG",
				Name:   "Engineering",
				Type:   "global",
				Status: "current",
				Homepage: &struct {
					ID string `json:"id"`
				}{ID: "100"},
				Icon: &struct {
					Path string `json:"path"`
				}{Path: "/images/eng-icon.png"},
			},
			{
				ID:     2,
				Key:    "HR",
				Name:   "Human Resources",
				Type:   "global",
				Status: "current",
			},
			{
				ID:     3,
				Key:    "PERSONAL",
				Name:   "My Space",
				Type:   "personal",
				Status: "current",
			},
		},
	}

	tests := []struct {
		name       string
		spaceType  string
		query      string
		limit      int
		wantCount  int
		wantKeys   []string
		wantParams map[string]string
	}{
		{
			name:      "returns all spaces with no filter",
			spaceType: "",
			query:     "",
			limit:     50,
			wantCount: 3,
			wantKeys:  []string{"ENG", "HR", "PERSONAL"},
		},
		{
			name:      "filters by query matching name",
			spaceType: "",
			query:     "engineer",
			limit:     50,
			wantCount: 1,
			wantKeys:  []string{"ENG"},
		},
		{
			name:      "filters by query matching key",
			spaceType: "",
			query:     "HR",
			limit:     50,
			wantCount: 1,
			wantKeys:  []string{"HR"},
		},
		{
			name:      "query filter is case-insensitive",
			spaceType: "",
			query:     "ENGINEERING",
			limit:     50,
			wantCount: 1,
			wantKeys:  []string{"ENG"},
		},
		{
			name:      "returns empty when query matches nothing",
			spaceType: "",
			query:     "nonexistent",
			limit:     50,
			wantCount: 0,
			wantKeys:  []string{},
		},
		{
			name:      "sends type parameter when spaceType provided",
			spaceType: "global",
			query:     "",
			limit:     50,
			wantCount: 3,
			wantParams: map[string]string{
				"type": "global",
			},
		},
		{
			name:      "defaults limit to 50 when zero",
			spaceType: "",
			query:     "",
			limit:     0,
			wantCount: 3,
			wantParams: map[string]string{
				"limit": "50",
			},
		},
		{
			name:      "defaults limit to 50 when negative",
			spaceType: "",
			query:     "",
			limit:     -1,
			wantCount: 3,
			wantParams: map[string]string{
				"limit": "50",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedParams map[string]string
			srv := newTestServer(t, map[string]http.HandlerFunc{
				"/rest/api/space": func(w http.ResponseWriter, r *http.Request) {
					capturedParams = make(map[string]string)
					for k, v := range r.URL.Query() {
						capturedParams[k] = v[0]
					}
					jsonResponse(t, w, spacesJSON)
				},
			})
			defer srv.Close()

			client := newTestClient(srv.URL)
			spaces, err := client.ListSpaces(context.Background(), tt.spaceType, tt.query, tt.limit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(spaces) != tt.wantCount {
				t.Errorf("got %d spaces, want %d", len(spaces), tt.wantCount)
			}

			for i, wantKey := range tt.wantKeys {
				if i >= len(spaces) {
					break
				}
				if spaces[i].Key != wantKey {
					t.Errorf("spaces[%d].Key = %q, want %q", i, spaces[i].Key, wantKey)
				}
			}

			for k, want := range tt.wantParams {
				if got := capturedParams[k]; got != want {
					t.Errorf("query param %q = %q, want %q", k, got, want)
				}
			}
		})
	}
}

func TestListSpaces_FieldMapping(t *testing.T) {
	spacesJSON := apiSpaceResponse{
		Results: []apiSpace{
			{
				ID:     42,
				Key:    "DEV",
				Name:   "Development",
				Type:   "global",
				Status: "current",
				Homepage: &struct {
					ID string `json:"id"`
				}{ID: "999"},
				Icon: &struct {
					Path string `json:"path"`
				}{Path: "/icons/dev.png"},
			},
		},
	}
	spacesJSON.Results[0].Description.Plain.Value = "Dev team docs"

	srv := newTestServer(t, map[string]http.HandlerFunc{
		"/rest/api/space": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(t, w, spacesJSON)
		},
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	spaces, err := client.ListSpaces(context.Background(), "", "", 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(spaces) != 1 {
		t.Fatalf("got %d spaces, want 1", len(spaces))
	}

	s := spaces[0]
	if s.ID != "42" {
		t.Errorf("ID = %q, want %q", s.ID, "42")
	}
	if s.Key != "DEV" {
		t.Errorf("Key = %q, want %q", s.Key, "DEV")
	}
	if s.Name != "Development" {
		t.Errorf("Name = %q, want %q", s.Name, "Development")
	}
	if s.Description != "Dev team docs" {
		t.Errorf("Description = %q, want %q", s.Description, "Dev team docs")
	}
	if s.Type != "global" {
		t.Errorf("Type = %q, want %q", s.Type, "global")
	}
	if s.Status != "current" {
		t.Errorf("Status = %q, want %q", s.Status, "current")
	}
	if s.HomepageID != "999" {
		t.Errorf("HomepageID = %q, want %q", s.HomepageID, "999")
	}
	wantIconURL := srv.URL + "/icons/dev.png"
	if s.IconURL != wantIconURL {
		t.Errorf("IconURL = %q, want %q", s.IconURL, wantIconURL)
	}
}

func TestListSpaces_NilHomepageAndIcon(t *testing.T) {
	spacesJSON := apiSpaceResponse{
		Results: []apiSpace{
			{
				ID:       10,
				Key:      "BARE",
				Name:     "Bare Space",
				Type:     "global",
				Homepage: nil,
				Icon:     nil,
			},
		},
	}

	srv := newTestServer(t, map[string]http.HandlerFunc{
		"/rest/api/space": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(t, w, spacesJSON)
		},
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	spaces, err := client.ListSpaces(context.Background(), "", "", 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(spaces) != 1 {
		t.Fatalf("got %d spaces, want 1", len(spaces))
	}
	if spaces[0].HomepageID != "" {
		t.Errorf("HomepageID = %q, want empty", spaces[0].HomepageID)
	}
	if spaces[0].IconURL != "" {
		t.Errorf("IconURL = %q, want empty", spaces[0].IconURL)
	}
}

func TestListSpaces_CachesResults(t *testing.T) {
	callCount := 0
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"/rest/api/space": func(w http.ResponseWriter, _ *http.Request) {
			callCount++
			jsonResponse(t, w, apiSpaceResponse{
				Results: []apiSpace{{ID: 1, Key: "A", Name: "Alpha"}},
			})
		},
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	ctx := context.Background()

	// First call hits server.
	_, err := client.ListSpaces(ctx, "", "", 50)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	// Second call should use cache.
	_, err = client.ListSpaces(ctx, "", "", 50)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if callCount != 1 {
		t.Errorf("server called %d times, want 1 (second call should be cached)", callCount)
	}
}

func TestListSpaces_ServerError(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"/rest/api/space": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"internal error"}`))
		},
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := client.ListSpaces(context.Background(), "", "", 50)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "listing spaces") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "listing spaces")
	}
}

func TestListSpaces_MalformedJSON(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"/rest/api/space": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{not valid json`))
		},
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := client.ListSpaces(context.Background(), "", "", 50)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
	if !strings.Contains(err.Error(), "decoding spaces response") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "decoding spaces response")
	}
}

// --- SearchPages ---

func TestSearchPages(t *testing.T) {
	searchResponse := apiSearchResponse{
		Results: []apiSearchResult{
			{
				Excerpt: "<b>Getting</b> started guide for new <em>engineers</em>",
			},
			{
				Excerpt: "Advanced configuration options",
			},
		},
		Start:     0,
		Limit:     25,
		Size:      2,
		TotalSize: 2,
	}
	searchResponse.Results[0].Content.ID = "101"
	searchResponse.Results[0].Content.Title = "Getting Started"
	searchResponse.Results[0].Content.Type = "page"
	searchResponse.Results[0].Content.Space.Key = "ENG"
	searchResponse.Results[0].Content.Links.WebUI = "/display/ENG/Getting+Started"
	searchResponse.Results[0].Content.Metadata = &struct {
		Labels struct {
			Results []struct {
				Name string `json:"name"`
			} `json:"results"`
		} `json:"labels"`
	}{
		Labels: struct {
			Results []struct {
				Name string `json:"name"`
			} `json:"results"`
		}{
			Results: []struct {
				Name string `json:"name"`
			}{
				{Name: "onboarding"},
				{Name: "guide"},
			},
		},
	}
	searchResponse.Results[1].Content.ID = "102"
	searchResponse.Results[1].Content.Title = "Configuration"
	searchResponse.Results[1].Content.Type = "page"
	searchResponse.Results[1].Content.Space.Key = "ENG"
	searchResponse.Results[1].Content.Links.WebUI = "/display/ENG/Configuration"

	tests := []struct {
		name      string
		params    SearchPagesParams
		wantCQL   string
		wantErr   bool
		wantCount int
	}{
		{
			name: "search with text query",
			params: SearchPagesParams{
				Query: "getting started",
				Limit: 25,
			},
			wantCQL:   `text~"getting started"`,
			wantCount: 2,
		},
		{
			name: "search with CQL",
			params: SearchPagesParams{
				CQL:   `title="Getting Started"`,
				Limit: 25,
			},
			wantCQL:   `title="Getting Started"`,
			wantCount: 2,
		},
		{
			name: "CQL takes precedence over query",
			params: SearchPagesParams{
				CQL:   `title="Test"`,
				Query: "ignored",
				Limit: 25,
			},
			wantCQL:   `title="Test"`,
			wantCount: 2,
		},
		{
			name: "space filter is added to CQL",
			params: SearchPagesParams{
				Query: "test",
				Space: "ENG",
				Limit: 25,
			},
			wantCQL:   `space="ENG" AND (text~"test")`,
			wantCount: 2,
		},
		{
			name: "space filter with CQL",
			params: SearchPagesParams{
				CQL:   `title="Test"`,
				Space: "ENG",
				Limit: 25,
			},
			wantCQL:   `space="ENG" AND (title="Test")`,
			wantCount: 2,
		},
		{
			name:    "returns error when no query or CQL",
			params:  SearchPagesParams{},
			wantErr: true,
		},
		{
			name: "defaults limit to 25 when zero",
			params: SearchPagesParams{
				Query: "test",
				Limit: 0,
			},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedCQL string
			srv := newTestServer(t, map[string]http.HandlerFunc{
				"/rest/api/content/search": func(w http.ResponseWriter, r *http.Request) {
					capturedCQL = r.URL.Query().Get("cql")
					jsonResponse(t, w, searchResponse)
				},
			})
			defer srv.Close()

			client := newTestClient(srv.URL)
			result, err := client.SearchPages(context.Background(), tt.params)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result.Pages) != tt.wantCount {
				t.Errorf("got %d pages, want %d", len(result.Pages), tt.wantCount)
			}
			if tt.wantCQL != "" && capturedCQL != tt.wantCQL {
				t.Errorf("CQL = %q, want %q", capturedCQL, tt.wantCQL)
			}
		})
	}
}

func TestSearchPages_FieldMapping(t *testing.T) {
	searchResponse := apiSearchResponse{
		Results: []apiSearchResult{
			{
				Excerpt: "<b>Hello</b> World",
			},
		},
		Start:     5,
		Limit:     10,
		TotalSize: 50,
	}
	searchResponse.Results[0].Content.ID = "200"
	searchResponse.Results[0].Content.Title = "Hello Page"
	searchResponse.Results[0].Content.Space.Key = "DEV"
	searchResponse.Results[0].Content.Links.WebUI = "/display/DEV/Hello"
	searchResponse.Results[0].Content.Metadata = &struct {
		Labels struct {
			Results []struct {
				Name string `json:"name"`
			} `json:"results"`
		} `json:"labels"`
	}{
		Labels: struct {
			Results []struct {
				Name string `json:"name"`
			} `json:"results"`
		}{
			Results: []struct {
				Name string `json:"name"`
			}{
				{Name: "doc"},
			},
		},
	}
	searchResponse.Links.Next = "/rest/api/content/search?cql=...&start=15"

	srv := newTestServer(t, map[string]http.HandlerFunc{
		"/rest/api/content/search": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(t, w, searchResponse)
		},
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	result, err := client.SearchPages(context.Background(), SearchPagesParams{
		Query: "hello",
		Limit: 10,
		Start: 5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 50 {
		t.Errorf("Total = %d, want 50", result.Total)
	}
	if result.Start != 5 {
		t.Errorf("Start = %d, want 5", result.Start)
	}
	if result.Limit != 10 {
		t.Errorf("Limit = %d, want 10", result.Limit)
	}
	if !result.HasMore {
		t.Error("HasMore = false, want true")
	}

	if len(result.Pages) != 1 {
		t.Fatalf("got %d pages, want 1", len(result.Pages))
	}
	p := result.Pages[0]
	if p.ID != "200" {
		t.Errorf("ID = %q, want %q", p.ID, "200")
	}
	if p.Title != "Hello Page" {
		t.Errorf("Title = %q, want %q", p.Title, "Hello Page")
	}
	if p.SpaceKey != "DEV" {
		t.Errorf("SpaceKey = %q, want %q", p.SpaceKey, "DEV")
	}
	if p.WebURL != "/display/DEV/Hello" {
		t.Errorf("WebURL = %q, want %q", p.WebURL, "/display/DEV/Hello")
	}
	if p.Excerpt != "Hello World" {
		t.Errorf("Excerpt = %q, want %q (HTML should be stripped)", p.Excerpt, "Hello World")
	}
	if len(p.Labels) != 1 || p.Labels[0] != "doc" {
		t.Errorf("Labels = %v, want [doc]", p.Labels)
	}
}

func TestSearchPages_NoMetadata(t *testing.T) {
	searchResponse := apiSearchResponse{
		Results: []apiSearchResult{
			{
				Excerpt: "plain excerpt",
			},
		},
		TotalSize: 1,
	}
	searchResponse.Results[0].Content.ID = "300"
	searchResponse.Results[0].Content.Title = "No Labels"
	searchResponse.Results[0].Content.Space.Key = "ENG"
	// Metadata is nil (no labels)

	srv := newTestServer(t, map[string]http.HandlerFunc{
		"/rest/api/content/search": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(t, w, searchResponse)
		},
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	result, err := client.SearchPages(context.Background(), SearchPagesParams{
		Query: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Pages[0].Labels) != 0 {
		t.Errorf("Labels = %v, want empty", result.Pages[0].Labels)
	}
}

func TestSearchPages_HasMoreFalseWhenNoNextLink(t *testing.T) {
	searchResponse := apiSearchResponse{
		Results:   []apiSearchResult{},
		TotalSize: 0,
	}
	// Links.Next is empty

	srv := newTestServer(t, map[string]http.HandlerFunc{
		"/rest/api/content/search": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(t, w, searchResponse)
		},
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	result, err := client.SearchPages(context.Background(), SearchPagesParams{
		Query: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.HasMore {
		t.Error("HasMore = true, want false when no next link")
	}
}

func TestSearchPages_CachesResults(t *testing.T) {
	callCount := 0
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"/rest/api/content/search": func(w http.ResponseWriter, _ *http.Request) {
			callCount++
			jsonResponse(t, w, apiSearchResponse{TotalSize: 0})
		},
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	ctx := context.Background()
	params := SearchPagesParams{Query: "cached-query", Limit: 10}

	_, _ = client.SearchPages(ctx, params)
	_, _ = client.SearchPages(ctx, params)

	if callCount != 1 {
		t.Errorf("server called %d times, want 1 (second call should be cached)", callCount)
	}
}

func TestSearchPages_ServerError(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"/rest/api/content/search": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"forbidden"}`))
		},
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := client.SearchPages(context.Background(), SearchPagesParams{Query: "test"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "searching pages") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "searching pages")
	}
}

func TestSearchPages_MalformedJSON(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"/rest/api/content/search": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{broken`))
		},
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := client.SearchPages(context.Background(), SearchPagesParams{Query: "test"})
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
	if !strings.Contains(err.Error(), "decoding search response") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "decoding search response")
	}
}

// --- ReadPage ---

func TestReadPage(t *testing.T) {
	pageJSON := apiContentResult{
		ID:    "500",
		Type:  "page",
		Title: "Architecture Overview",
	}
	pageJSON.Space.Key = "ENG"
	pageJSON.Body.Storage.Value = "<p>Hello <strong>world</strong></p>"
	pageJSON.Version.Number = 7
	pageJSON.Version.When = "2025-01-15T10:30:00.000Z"
	pageJSON.History.CreatedDate = "2024-06-01T08:00:00.000Z"
	pageJSON.Links.WebUI = "/display/ENG/Architecture+Overview"
	pageJSON.Ancestors = []struct {
		ID string `json:"id"`
	}{
		{ID: "10"},
		{ID: "20"},
		{ID: "30"},
	}
	pageJSON.Metadata = &struct {
		Labels struct {
			Results []struct {
				Name string `json:"name"`
			} `json:"results"`
		} `json:"labels"`
	}{
		Labels: struct {
			Results []struct {
				Name string `json:"name"`
			} `json:"results"`
		}{
			Results: []struct {
				Name string `json:"name"`
			}{
				{Name: "architecture"},
				{Name: "design"},
			},
		},
	}

	srv := newTestServer(t, map[string]http.HandlerFunc{
		"/rest/api/content/500": func(w http.ResponseWriter, r *http.Request) {
			expand := r.URL.Query().Get("expand")
			if expand == "" {
				t.Error("expand query parameter not set")
			}
			jsonResponse(t, w, pageJSON)
		},
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	page, err := client.ReadPage(context.Background(), "500")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if page.ID != "500" {
		t.Errorf("ID = %q, want %q", page.ID, "500")
	}
	if page.Title != "Architecture Overview" {
		t.Errorf("Title = %q, want %q", page.Title, "Architecture Overview")
	}
	if page.SpaceKey != "ENG" {
		t.Errorf("SpaceKey = %q, want %q", page.SpaceKey, "ENG")
	}
	if page.Version != 7 {
		t.Errorf("Version = %d, want 7", page.Version)
	}
	if page.UpdatedAt != "2025-01-15T10:30:00.000Z" {
		t.Errorf("UpdatedAt = %q, want %q", page.UpdatedAt, "2025-01-15T10:30:00.000Z")
	}
	if page.CreatedAt != "2024-06-01T08:00:00.000Z" {
		t.Errorf("CreatedAt = %q, want %q", page.CreatedAt, "2024-06-01T08:00:00.000Z")
	}
	if page.WebURL != "/display/ENG/Architecture+Overview" {
		t.Errorf("WebURL = %q, want %q", page.WebURL, "/display/ENG/Architecture+Overview")
	}
	// Ancestors: [10, 20, 30], ParentID = last ancestor = 30
	if len(page.Ancestors) != 3 {
		t.Errorf("len(Ancestors) = %d, want 3", len(page.Ancestors))
	}
	if page.ParentID != "30" {
		t.Errorf("ParentID = %q, want %q (last ancestor)", page.ParentID, "30")
	}
	if len(page.Labels) != 2 || page.Labels[0] != "architecture" || page.Labels[1] != "design" {
		t.Errorf("Labels = %v, want [architecture design]", page.Labels)
	}
	// Content should be converted from storage format to markdown
	if !strings.Contains(page.Content, "**world**") {
		t.Errorf("Content = %q, want it to contain markdown bold **world**", page.Content)
	}
}

func TestReadPage_NoAncestors(t *testing.T) {
	pageJSON := apiContentResult{
		ID:    "600",
		Title: "Root Page",
	}
	pageJSON.Space.Key = "ENG"
	pageJSON.Body.Storage.Value = "<p>content</p>"
	pageJSON.Version.Number = 1

	srv := newTestServer(t, map[string]http.HandlerFunc{
		"/rest/api/content/600": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(t, w, pageJSON)
		},
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	page, err := client.ReadPage(context.Background(), "600")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if page.ParentID != "" {
		t.Errorf("ParentID = %q, want empty for root page", page.ParentID)
	}
	if len(page.Ancestors) != 0 {
		t.Errorf("Ancestors = %v, want empty", page.Ancestors)
	}
}

func TestReadPage_NoMetadata(t *testing.T) {
	pageJSON := apiContentResult{
		ID:    "700",
		Title: "No Labels Page",
	}
	pageJSON.Space.Key = "ENG"
	pageJSON.Body.Storage.Value = "<p>text</p>"
	pageJSON.Version.Number = 1
	// Metadata is nil

	srv := newTestServer(t, map[string]http.HandlerFunc{
		"/rest/api/content/700": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(t, w, pageJSON)
		},
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	page, err := client.ReadPage(context.Background(), "700")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(page.Labels) != 0 {
		t.Errorf("Labels = %v, want empty", page.Labels)
	}
}

func TestReadPage_CachesResults(t *testing.T) {
	callCount := 0
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"/rest/api/content/123": func(w http.ResponseWriter, _ *http.Request) {
			callCount++
			pageJSON := apiContentResult{ID: "123", Title: "Cached"}
			pageJSON.Space.Key = "ENG"
			pageJSON.Version.Number = 1
			jsonResponse(t, w, pageJSON)
		},
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	ctx := context.Background()

	_, _ = client.ReadPage(ctx, "123")
	_, _ = client.ReadPage(ctx, "123")

	if callCount != 1 {
		t.Errorf("server called %d times, want 1", callCount)
	}
}

func TestReadPage_ServerError(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"/rest/api/content/404": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"not found"}`))
		},
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := client.ReadPage(context.Background(), "404")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "reading page 404") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "reading page 404")
	}
}

func TestReadPage_MalformedJSON(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"/rest/api/content/bad": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`not-json`))
		},
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := client.ReadPage(context.Background(), "bad")
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
	if !strings.Contains(err.Error(), "decoding page bad") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "decoding page bad")
	}
}

// --- ReadPageByTitle ---

func TestReadPageByTitle(t *testing.T) {
	contentResponse := apiContentResponse{
		Results: []apiContentResult{
			{
				ID:    "800",
				Title: "Release Notes",
			},
		},
	}
	contentResponse.Results[0].Space.Key = "ENG"
	contentResponse.Results[0].Body.Storage.Value = "<p>v2.0 released</p>"
	contentResponse.Results[0].Version.Number = 3
	contentResponse.Results[0].Version.When = "2025-02-01T12:00:00.000Z"

	tests := []struct {
		name     string
		spaceKey string
		title    string
		wantErr  bool
	}{
		{
			name:     "finds page by title",
			spaceKey: "ENG",
			title:    "Release Notes",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedSpaceKey, capturedTitle string
			srv := newTestServer(t, map[string]http.HandlerFunc{
				"/rest/api/content": func(w http.ResponseWriter, r *http.Request) {
					capturedSpaceKey = r.URL.Query().Get("spaceKey")
					capturedTitle = r.URL.Query().Get("title")
					jsonResponse(t, w, contentResponse)
				},
			})
			defer srv.Close()

			client := newTestClient(srv.URL)
			page, err := client.ReadPageByTitle(context.Background(), tt.spaceKey, tt.title)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if capturedSpaceKey != tt.spaceKey {
				t.Errorf("spaceKey param = %q, want %q", capturedSpaceKey, tt.spaceKey)
			}
			if capturedTitle != tt.title {
				t.Errorf("title param = %q, want %q", capturedTitle, tt.title)
			}
			if page.ID != "800" {
				t.Errorf("ID = %q, want %q", page.ID, "800")
			}
			if page.Title != "Release Notes" {
				t.Errorf("Title = %q, want %q", page.Title, "Release Notes")
			}
		})
	}
}

func TestReadPageByTitle_NotFound(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"/rest/api/content": func(w http.ResponseWriter, _ *http.Request) {
			jsonResponse(t, w, apiContentResponse{Results: []apiContentResult{}})
		},
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := client.ReadPageByTitle(context.Background(), "ENG", "Does Not Exist")
	if err == nil {
		t.Fatal("expected error for missing page, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "not found")
	}
}

func TestReadPageByTitle_ServerError(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"/rest/api/content": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"error"}`))
		},
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := client.ReadPageByTitle(context.Background(), "ENG", "Test")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "reading page by title") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "reading page by title")
	}
}

func TestReadPageByTitle_MalformedJSON(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"/rest/api/content": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{{invalid`))
		},
	})
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := client.ReadPageByTitle(context.Background(), "ENG", "Test")
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
	if !strings.Contains(err.Error(), "decoding page-by-title response") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "decoding page-by-title response")
	}
}

// --- doGet error handling ---

func TestDoGet_CanceledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := client.Health(ctx)
	if err == nil {
		t.Fatal("expected error for canceled context, got nil")
	}
}

func TestDoGet_ServerDown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	// Close immediately to simulate unreachable server.
	srv.Close()

	client := newTestClient(srv.URL)
	err := client.Health(context.Background())
	if err == nil {
		t.Fatal("expected error for closed server, got nil")
	}
}

func TestDoGet_ErrorResponseIncludesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"bad request detail"}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	err := client.Health(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error = %q, want it to contain status code 400", err.Error())
	}
	if !strings.Contains(err.Error(), "bad request detail") {
		t.Errorf("error = %q, want it to contain response body", err.Error())
	}
}

// --- NewClient ---

func TestNewClient_Validation(t *testing.T) {
	t.Run("rejects localhost URL", func(t *testing.T) {
		_, err := NewClient("http://localhost:8080", "user", "token", slog.Default())
		if err == nil {
			t.Fatal("expected error for localhost URL")
		}
	})

	t.Run("allows private IP URL for internal services", func(t *testing.T) {
		_, err := NewClient("http://192.168.1.1:8080", "user", "token", slog.Default())
		if err != nil {
			t.Fatalf("expected no error for private IP URL, got: %v", err)
		}
	})

	t.Run("rejects loopback IP URL", func(t *testing.T) {
		_, err := NewClient("http://127.0.0.1:8080", "user", "token", slog.Default())
		if err == nil {
			t.Fatal("expected error for loopback IP URL")
		}
	})
}

// --- buildSearchCQL ---

func TestBuildSearchCQL(t *testing.T) {
	tests := []struct {
		name   string
		params SearchPagesParams
		want   string
	}{
		{
			name:   "empty params returns empty",
			params: SearchPagesParams{},
			want:   "",
		},
		{
			name:   "query only",
			params: SearchPagesParams{Query: "hello"},
			want:   `text~"hello"`,
		},
		{
			name:   "CQL only",
			params: SearchPagesParams{CQL: `type=page`},
			want:   `type=page`,
		},
		{
			name:   "CQL takes precedence over query",
			params: SearchPagesParams{CQL: `type=page`, Query: "hello"},
			want:   `type=page`,
		},
		{
			name:   "query with space filter",
			params: SearchPagesParams{Query: "hello", Space: "ENG"},
			want:   `space="ENG" AND (text~"hello")`,
		},
		{
			name:   "CQL with space filter",
			params: SearchPagesParams{CQL: `type=page`, Space: "DEV"},
			want:   `space="DEV" AND (type=page)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSearchCQL(tt.params)
			if got != tt.want {
				t.Errorf("buildSearchCQL() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- Helper functions ---

func TestStripHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no HTML",
			input: "plain text",
			want:  "plain text",
		},
		{
			name:  "simple tags",
			input: "<b>bold</b> and <em>italic</em>",
			want:  "bold and italic",
		},
		{
			name:  "nested tags",
			input: "<div><p><b>deep</b></p></div>",
			want:  "deep",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "only tags",
			input: "<br/><hr/>",
			want:  "",
		},
		{
			name:  "preserves text between tags",
			input: "before <span>middle</span> after",
			want:  "before middle after",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripHTML(tt.input)
			if got != tt.want {
				t.Errorf("stripHTML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		n     int
		want  string
	}{
		{
			name:  "short string unchanged",
			input: "hello",
			n:     10,
			want:  "hello",
		},
		{
			name:  "exact length unchanged",
			input: "hello",
			n:     5,
			want:  "hello",
		},
		{
			name:  "truncated with ellipsis",
			input: "hello world",
			n:     5,
			want:  "hello...",
		},
		{
			name:  "empty string",
			input: "",
			n:     5,
			want:  "",
		},
		{
			name:  "zero limit",
			input: "hello",
			n:     0,
			want:  "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.n)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
			}
		})
	}
}

func TestFilterSpaces(t *testing.T) {
	spaces := []Space{
		{Key: "ENG", Name: "Engineering"},
		{Key: "HR", Name: "Human Resources"},
		{Key: "SALES", Name: "Sales Team"},
	}

	tests := []struct {
		name     string
		query    string
		wantKeys []string
	}{
		{
			name:     "empty query returns all",
			query:    "",
			wantKeys: []string{"ENG", "HR", "SALES"},
		},
		{
			name:     "matches name substring",
			query:    "engineer",
			wantKeys: []string{"ENG"},
		},
		{
			name:     "matches key substring",
			query:    "HR",
			wantKeys: []string{"HR"},
		},
		{
			name:     "case insensitive match",
			query:    "sales",
			wantKeys: []string{"SALES"},
		},
		{
			name:     "no matches returns empty",
			query:    "finance",
			wantKeys: []string{},
		},
		{
			name:     "partial key match",
			query:    "EN",
			wantKeys: []string{"ENG"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterSpaces(spaces, tt.query)
			if len(got) != len(tt.wantKeys) {
				t.Fatalf("got %d spaces, want %d", len(got), len(tt.wantKeys))
			}
			for i, wantKey := range tt.wantKeys {
				if got[i].Key != wantKey {
					t.Errorf("got[%d].Key = %q, want %q", i, got[i].Key, wantKey)
				}
			}
		})
	}
}
