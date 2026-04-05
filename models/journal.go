package models

// Author represents a journal author
type Author struct {
	Name        string `json:"name"`
	Affiliation string `json:"affiliation,omitempty"`
}

// Journal represents a journal article
type Journal struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Authors     []Author `json:"authors"`
	Abstract    string   `json:"abstract"`
	Source      string   `json:"source"`
	Year        int      `json:"year"`
	Language    string   `json:"language"`
	Subjects    []string `json:"subjects"`
	DOI         string   `json:"doi,omitempty"`
	URL         string   `json:"url"`
	PDFURL      string   `json:"pdf_url,omitempty"`
	IsOpenAccess bool    `json:"is_open_access"`
	Citations   int      `json:"citations,omitempty"`
	PublishedAt string   `json:"published_at"`
	Score       float64  `json:"score,omitempty"` // Internal ranking score
}

// SearchRequest represents a search request
type SearchRequest struct {
	Query    string   `json:"query"`
	Page     int      `json:"page"`
	PageSize int      `json:"page_size"`
	YearFrom int      `json:"year_from,omitempty"`
	YearTo   int      `json:"year_to,omitempty"`
	Language string   `json:"language,omitempty"`
	Subjects []string `json:"subjects,omitempty"`
	SortBy   string   `json:"sort_by,omitempty"`
	Source   string   `json:"source,omitempty"`
}

// SearchResponse represents a search response
type SearchResponse struct {
	Results    []Journal `json:"results"`
	Total      int       `json:"total"`
	Page       int       `json:"page"`
	PageSize   int       `json:"page_size"`
	Source     string    `json:"source"`
}

// AggregatedResponse represents aggregated search results from multiple sources
type AggregatedResponse struct {
	Results    []Journal `json:"results"`
	Total      int       `json:"total"`
	Page       int       `json:"page"`
	PageSize   int       `json:"page_size"`
	Sources    []string  `json:"sources"`
	Query      string    `json:"query"`
}