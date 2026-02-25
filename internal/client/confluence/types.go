package confluence

// Space represents a Confluence space from the REST API.
type Space struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"` // global, personal
	Status      string `json:"status,omitempty"`
	HomepageID  string `json:"homepage_id,omitempty"`
	IconURL     string `json:"icon_url,omitempty"`
}

// SearchPagesParams holds parameters for page search.
type SearchPagesParams struct {
	CQL   string
	Query string
	Space string
	Limit int
	Start int
}

// SearchResult holds paged search results.
type SearchResult struct {
	Pages   []PageSummary
	Total   int
	Start   int
	Limit   int
	HasMore bool
	CQL     string
}

// PageSummary is a lightweight page representation in search results.
type PageSummary struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	SpaceKey string   `json:"space_key"`
	WebURL   string   `json:"web_url"`
	Excerpt  string   `json:"excerpt,omitempty"`
	Labels   []string `json:"labels,omitempty"`
}

// Page is a full Confluence page with content.
type Page struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	SpaceKey  string   `json:"space_key"`
	ParentID  string   `json:"parent_id,omitempty"`
	WebURL    string   `json:"web_url"`
	Version   int      `json:"version"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
	Labels    []string `json:"labels,omitempty"`
	Ancestors []string `json:"ancestors,omitempty"` // ancestor page IDs
	Content   string   `json:"content"`             // Markdown content
}

// apiSpaceResponse is the raw JSON envelope returned by the Confluence spaces endpoint.
type apiSpaceResponse struct {
	Results []apiSpace `json:"results"`
}

// apiSpace is a single space in the Confluence REST API response.
type apiSpace struct {
	ID          int    `json:"id"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	Description struct {
		Plain struct {
			Value string `json:"value"`
		} `json:"plain"`
	} `json:"description"`
	Homepage *struct {
		ID string `json:"id"`
	} `json:"homepage"`
	Icon *struct {
		Path string `json:"path"`
	} `json:"icon"`
	Links struct {
		WebUI string `json:"webui"`
	} `json:"_links"`
}

// apiSearchResponse is the raw JSON envelope returned by the Confluence search endpoint.
type apiSearchResponse struct {
	Results []apiSearchResult `json:"results"`
	Start   int               `json:"start"`
	Limit   int               `json:"limit"`
	Size    int               `json:"size"`
	Links   struct {
		Next string `json:"next"`
	} `json:"_links"`
	TotalSize int `json:"totalSize"`
}

// apiSearchResult is a single result in the Confluence search response.
type apiSearchResult struct {
	Content struct {
		ID    string `json:"id"`
		Type  string `json:"type"`
		Title string `json:"title"`
		Space struct {
			Key string `json:"key"`
		} `json:"space"`
		Metadata *struct {
			Labels struct {
				Results []struct {
					Name string `json:"name"`
				} `json:"results"`
			} `json:"labels"`
		} `json:"metadata"`
		Links struct {
			WebUI string `json:"webui"`
		} `json:"_links"`
	} `json:"content"`
	Excerpt string `json:"excerpt"`
}

// apiContentResponse is the raw JSON envelope for content list queries.
type apiContentResponse struct {
	Results []apiContentResult `json:"results"`
}

// apiContentResult is a single page in the Confluence content response.
type apiContentResult struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
	Space struct {
		Key string `json:"key"`
	} `json:"space"`
	Body struct {
		Storage struct {
			Value string `json:"value"`
		} `json:"storage"`
	} `json:"body"`
	Version struct {
		Number int    `json:"number"`
		When   string `json:"when"`
	} `json:"version"`
	Ancestors []struct {
		ID string `json:"id"`
	} `json:"ancestors"`
	Metadata *struct {
		Labels struct {
			Results []struct {
				Name string `json:"name"`
			} `json:"results"`
		} `json:"labels"`
	} `json:"metadata"`
	History struct {
		CreatedDate string `json:"createdDate"`
	} `json:"history"`
	Links struct {
		WebUI string `json:"webui"`
		Base  string `json:"base"`
	} `json:"_links"`
}
