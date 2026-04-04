package services

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"jurnalinal/models"
)

// ArXivService handles arXiv API requests
type ArXivService struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewArXivService creates a new arXiv service
func NewArXivService() *ArXivService {
	return &ArXivService{
		BaseURL: "http://export.arxiv.org/api/query",
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ArXivResponse represents the arXiv API response structure
type ArXivResponse struct {
	XMLName   xml.Name `xml:"feed"`
	Total     string   `xml:"opensearch:totalResults"`
	Entries   []Entry  `xml:"entry"`
}

// Entry represents an arXiv entry
type Entry struct {
	ID        string    `xml:"id"`
	Title     string    `xml:"title"`
	Summary   string    `xml:"summary"`
	Published string    `xml:"published"`
	Updated   string    `xml:"updated"`
	Authors   []Author  `xml:"author"`
	Links     []Link    `xml:"link"`
	Categories []Category `xml:"category"`
}

// Author represents an arXiv author
type Author struct {
	Name string `xml:"name"`
}

// Link represents an arXiv link
type Link struct {
	Href string `xml:"href,attr"`
	Type string `xml:"type,attr,omitempty"`
	Rel  string `xml:"rel,attr,omitempty"`
}

// Category represents an arXiv category
type Category struct {
	Term string `xml:"term,attr"`
}

// Search searches for papers in arXiv
func (s *ArXivService) Search(req models.SearchRequest) (models.SearchResponse, error) {
	resp := models.SearchResponse{
		Source:   "arXiv",
		Page:     req.Page,
		PageSize: req.PageSize,
	}

	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}

	start := (req.Page - 1) * req.PageSize

	// Build query URL
	queryURL := fmt.Sprintf("%s?search_query=all:%s&start=%d&max_results=%d&sortBy=relevance&sortOrder=descending",
		s.BaseURL,
		url.QueryEscape(req.Query),
		start,
		req.PageSize,
	)

	// Make HTTP request
	httpResp, err := s.HTTPClient.Get(queryURL)
	if err != nil {
		return resp, fmt.Errorf("failed to execute request: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return resp, fmt.Errorf("failed to read response: %w", err)
	}

	var arxivResp ArXivResponse
	if err := xml.Unmarshal(body, &arxivResp); err != nil {
		return resp, fmt.Errorf("failed to parse response: %w", err)
	}

	total, _ := strconv.Atoi(arxivResp.Total)
	resp.Total = total

	// Convert to our model
	for _, entry := range arxivResp.Entries {
		journal := models.Journal{
			Source:       "arXiv",
			IsOpenAccess: true, // arXiv is always open access
		}

		// Title
		journal.Title = strings.TrimSpace(entry.Title)
		journal.Title = strings.ReplaceAll(journal.Title, "\n", " ")
		for strings.Contains(journal.Title, "  ") {
			journal.Title = strings.ReplaceAll(journal.Title, "  ", " ")
		}

		// Authors
		for _, auth := range entry.Authors {
			if auth.Name != "" {
				journal.Authors = append(journal.Authors, models.Author{
					Name: auth.Name,
				})
			}
		}

		// Abstract
		journal.Abstract = strings.TrimSpace(entry.Summary)
		journal.Abstract = strings.ReplaceAll(journal.Abstract, "\n", " ")

		// Year from published date
		if entry.Published != "" {
			if t, err := time.Parse("2006-01-02T15:04:05Z", entry.Published); err == nil {
				journal.Year = t.Year()
				journal.PublishedAt = entry.Published
			}
		}

		// ID (extract arXiv ID)
		journal.ID = "arxiv_" + extractArXivID(entry.ID)

		// URL and PDF URL
		arxivID := extractArXivID(entry.ID)
		journal.URL = fmt.Sprintf("https://arxiv.org/abs/%s", arxivID)
		journal.PDFURL = fmt.Sprintf("https://arxiv.org/pdf/%s", arxivID)

		// DOI (if available)
		for _, link := range entry.Links {
			if link.Rel == "related" && strings.Contains(link.Href, "doi.org") {
				journal.DOI = strings.TrimPrefix(link.Href, "https://doi.org/")
			}
		}

		// Language
		journal.Language = "en"

		// Subjects (categories)
		for _, cat := range entry.Categories {
			if cat.Term != "" {
				journal.Subjects = append(journal.Subjects, cat.Term)
			}
		}

		resp.Results = append(resp.Results, journal)
	}

	return resp, nil
}

// extractArXivID extracts the arXiv ID from the full ID URL
func extractArXivID(id string) string {
	// Format: http://arxiv.org/abs/2101.12345v1 or http://arxiv.org/abs/1234.56789v1
	parts := strings.Split(id, "/abs/")
	if len(parts) > 1 {
		return strings.Split(parts[1], "v")[0]
	}
	parts = strings.Split(id, "/")
	return parts[len(parts)-1]
}

// GetDetail gets paper detail by arXiv ID
func (s *ArXivService) GetDetail(arxivID string) (models.Journal, error) {
	var journal models.Journal

	queryURL := fmt.Sprintf("%s?id_list=%s", s.BaseURL, url.PathEscape(arxivID))

	httpResp, err := s.HTTPClient.Get(queryURL)
	if err != nil {
		return journal, fmt.Errorf("failed to execute request: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return journal, fmt.Errorf("failed to read response: %w", err)
	}

	var arxivResp ArXivResponse
	if err := xml.Unmarshal(body, &arxivResp); err != nil {
		return journal, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(arxivResp.Entries) == 0 {
		return journal, fmt.Errorf("paper not found")
	}

	entry := arxivResp.Entries[0]

	// Title
	journal.Title = strings.TrimSpace(entry.Title)
	journal.Title = strings.ReplaceAll(journal.Title, "\n", " ")

	// Authors
	for _, auth := range entry.Authors {
		if auth.Name != "" {
			journal.Authors = append(journal.Authors, models.Author{
				Name: auth.Name,
			})
		}
	}

	// Abstract
	journal.Abstract = strings.TrimSpace(entry.Summary)

	// Year
	if entry.Published != "" {
		if t, err := time.Parse("2006-01-02T15:04:05Z", entry.Published); err == nil {
			journal.Year = t.Year()
			journal.PublishedAt = entry.Published
		}
	}

	// ID, URL, PDF
	journal.ID = "arxiv_" + extractArXivID(entry.ID)
	journal.URL = fmt.Sprintf("https://arxiv.org/abs/%s", extractArXivID(entry.ID))
	journal.PDFURL = fmt.Sprintf("https://arxiv.org/pdf/%s", extractArXivID(entry.ID))
	journal.IsOpenAccess = true
	journal.Source = "arXiv"
	journal.Language = "en"

	// Subjects
	for _, cat := range entry.Categories {
		if cat.Term != "" {
			journal.Subjects = append(journal.Subjects, cat.Term)
		}
	}

	return journal, nil
}

// GetName returns the service name
func (s *ArXivService) GetName() string {
	return "arXiv"
}

// IsAvailable checks if the service is available
func (s *ArXivService) IsAvailable() bool {
	resp, err := s.HTTPClient.Get(s.BaseURL + "?search_query=test&max_results=1")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}