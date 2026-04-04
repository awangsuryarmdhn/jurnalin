package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"jurnalinal/models"
)

// CrossRefService handles CrossRef API requests
type CrossRefService struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewCrossRefService creates a new CrossRef service
func NewCrossRefService() *CrossRefService {
	return &CrossRefService{
		BaseURL: "https://api.crossref.org",
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CrossRefResponse represents the CrossRef API response structure
type CrossRefResponse struct {
	Status   string `json:"status"`
	Message  struct {
		TotalResults int `json:"total-results"`
		Items        []struct {
			Title         []string `json:"title"`
			Author        []struct {
				Given  string `json:"given"`
				Family string `json:"family"`
				Name   string `json:"name"`
			} `json:"author,omitempty"`
			Abstract    string `json:"abstract,omitempty"`
			Published   struct {
				DateParts [][]int `json:"date-parts"`
			} `json:"published,omitempty"`
			Created struct {
				DateParts [][]int `json:"date-parts"`
			} `json:"created,omitempty"`
			DOI         string   `json:"DOI"`
			URL         string   `json:"URL"`
			Language    string   `json:"language,omitempty"`
			Subject     []string `json:"subject,omitempty"`
			Link        []struct {
				URL         string `json:"URL"`
				ContentType string `json:"content-type,omitempty"`
			} `json:"link,omitempty"`
			IsReferencedByCount int `json:"is-referenced-by-count,omitempty"`
			License             []struct {
				URL string `json:"URL,omitempty"`
			} `json:"license,omitempty"`
		} `json:"items"`
	} `json:"message"`
}

// Search searches for journals in CrossRef
func (s *CrossRefService) Search(req models.SearchRequest) (models.SearchResponse, error) {
	resp := models.SearchResponse{
		Source:   "CrossRef",
		Page:     req.Page,
		PageSize: req.PageSize,
	}

	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}

	// Build query URL
	queryURL := fmt.Sprintf("%s/works?query=%s&rows=%d&offset=%d&select=title,author,abstract,published,created,DOI,URL,subject,link,is-referenced-by-count,license",
		s.BaseURL,
		url.QueryEscape(req.Query),
		req.PageSize,
		(req.Page-1)*req.PageSize,
	)

	// Make HTTP request
	httpReq, err := http.NewRequest("GET", queryURL, nil)
	if err != nil {
		return resp, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("User-Agent", "JURNALIN/1.0 (mailto:admin@jurnalinal.com)")

	httpResp, err := s.HTTPClient.Do(httpReq)
	if err != nil {
		return resp, fmt.Errorf("failed to execute request: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return resp, fmt.Errorf("failed to read response: %w", err)
	}

	var crossResp CrossRefResponse
	if err := json.Unmarshal(body, &crossResp); err != nil {
		return resp, fmt.Errorf("failed to parse response: %w", err)
	}

	resp.Total = crossResp.Message.TotalResults

	// Convert to our model
	for _, item := range crossResp.Message.Items {
		journal := models.Journal{
			Source: "CrossRef",
		}

		// Title
		if len(item.Title) > 0 {
			journal.Title = item.Title[0]
		}

		// Authors
		for _, auth := range item.Author {
			name := auth.Name
			if name == "" {
				name = strings.TrimSpace(auth.Given + " " + auth.Family)
			}
			if name != "" {
				journal.Authors = append(journal.Authors, models.Author{
					Name: name,
				})
			}
		}

		// Abstract (clean HTML tags)
		journal.Abstract = cleanHTML(item.Abstract)

		// Year
		if len(item.Published.DateParts) > 0 && len(item.Published.DateParts[0]) > 0 {
			journal.Year = item.Published.DateParts[0][0]
		} else if len(item.Created.DateParts) > 0 && len(item.Created.DateParts[0]) > 0 {
			journal.Year = item.Created.DateParts[0][0]
		}

		// DOI and URL
		journal.DOI = item.DOI
		journal.URL = item.URL
		if journal.URL == "" && journal.DOI != "" {
			journal.URL = fmt.Sprintf("https://doi.org/%s", journal.DOI)
		}

		// PDF URL
		for _, link := range item.Link {
			if strings.Contains(link.ContentType, "pdf") {
				journal.PDFURL = link.URL
				break
			}
		}

		// Language
		journal.Language = item.Language
		if journal.Language == "" {
			journal.Language = "en"
		}

		// Subjects
		journal.Subjects = item.Subject

		// Citations
		journal.Citations = item.IsReferencedByCount

		// Open Access check
		journal.IsOpenAccess = len(item.License) > 0

		// ID
		journal.ID = "crossref_" + journal.DOI

		resp.Results = append(resp.Results, journal)
	}

	return resp, nil
}

// cleanHTML removes HTML tags from a string
func cleanHTML(s string) string {
	// Simple HTML tag removal
	for {
		start := strings.Index(s, "<")
		if start == -1 {
			break
		}
		end := strings.Index(s[start:], ">")
		if end == -1 {
			break
		}
		s = s[:start] + s[start+end+1:]
	}
	// Decode common HTML entities
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	// Clean up multiple spaces
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

// GetDetail gets journal detail by DOI
func (s *CrossRefService) GetDetail(doi string) (models.Journal, error) {
	var journal models.Journal

	queryURL := fmt.Sprintf("%s/works/%s", s.BaseURL, url.PathEscape(doi))

	httpReq, err := http.NewRequest("GET", queryURL, nil)
	if err != nil {
		return journal, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("User-Agent", "JURNALIN/1.0 (mailto:admin@jurnalinal.com)")

	httpResp, err := s.HTTPClient.Do(httpReq)
	if err != nil {
		return journal, fmt.Errorf("failed to execute request: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return journal, fmt.Errorf("failed to read response: %w", err)
	}

	var crossResp struct {
		Status  string `json:"status"`
		Message struct {
			Title         []string `json:"title"`
			Author        []struct {
				Given  string `json:"given"`
				Family string `json:"family"`
				Name   string `json:"name"`
			} `json:"author,omitempty"`
			Abstract    string `json:"abstract,omitempty"`
			Published   struct {
				DateParts [][]int `json:"date-parts"`
			} `json:"published,omitempty"`
			DOI         string   `json:"DOI"`
			URL         string   `json:"URL"`
			Language    string   `json:"language,omitempty"`
			Subject     []string `json:"subject,omitempty"`
			Link        []struct {
				URL         string `json:"URL"`
				ContentType string `json:"content-type,omitempty"`
			} `json:"link,omitempty"`
			IsReferencedByCount int `json:"is-referenced-by-count,omitempty"`
			License             []struct {
				URL string `json:"URL,omitempty"`
			} `json:"license,omitempty"`
		} `json:"message"`
	}

	if err := json.Unmarshal(body, &crossResp); err != nil {
		return journal, fmt.Errorf("failed to parse response: %w", err)
	}

	item := crossResp.Message

	// Title
	if len(item.Title) > 0 {
		journal.Title = item.Title[0]
	}

	// Authors
	for _, auth := range item.Author {
		name := auth.Name
		if name == "" {
			name = strings.TrimSpace(auth.Given + " " + auth.Family)
		}
		if name != "" {
			journal.Authors = append(journal.Authors, models.Author{
				Name: name,
			})
		}
	}

	// Abstract
	journal.Abstract = cleanHTML(item.Abstract)

	// Year
	if len(item.Published.DateParts) > 0 && len(item.Published.DateParts[0]) > 0 {
		journal.Year = item.Published.DateParts[0][0]
	}

	// DOI and URL
	journal.DOI = item.DOI
	journal.URL = item.URL
	if journal.URL == "" && journal.DOI != "" {
		journal.URL = fmt.Sprintf("https://doi.org/%s", journal.DOI)
	}

	// PDF URL
	for _, link := range item.Link {
		if strings.Contains(link.ContentType, "pdf") {
			journal.PDFURL = link.URL
			break
		}
	}

	// Language
	journal.Language = item.Language
	if journal.Language == "" {
		journal.Language = "en"
	}

	// Subjects
	journal.Subjects = item.Subject

	// Citations
	journal.Citations = item.IsReferencedByCount

	// Open Access
	journal.IsOpenAccess = len(item.License) > 0

	// Source
	journal.Source = "CrossRef"
	journal.ID = "crossref_" + journal.DOI

	return journal, nil
}

// GetName returns the service name
func (s *CrossRefService) GetName() string {
	return "CrossRef"
}

// IsAvailable checks if the service is available
func (s *CrossRefService) IsAvailable() bool {
	resp, err := s.HTTPClient.Get(s.BaseURL + "/works?query=test&rows=1")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// FormatYear converts year to string for URL params
func FormatYear(year int) string {
	if year > 0 {
		return strconv.Itoa(year)
	}
	return ""
}