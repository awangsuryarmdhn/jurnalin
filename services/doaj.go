package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"jurnalinal/models"
)

// DOAJService handles DOAJ API requests
type DOAJService struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewDOAJService creates a new DOAJ service
func NewDOAJService() *DOAJService {
	return &DOAJService{
		BaseURL: "https://doaj.org/api/search/articles",
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// DOAJResponse represents the DOAJ API response structure
type DOAJResponse struct {
	Total     int `json:"total"`
	Results   []DOAJArticle `json:"results"`
}

// DOAJArticle represents a DOAJ article
type DOAJArticle struct {
	Bibjson struct {
		Title       string   `json:"title"`
		Abstract    string   `json:"abstract"`
		Year        string   `json:"year"`
		Language    []string `json:"language"`
		Subject     []struct {
			Term string `json:"term"`
		} `json:"subject"`
		Link        []struct {
			URL         string `json:"url"`
			Type        string `json:"type"`
			ContentType string `json:"content_type"`
		} `json:"link"`
		Author []struct {
			Name string `json:"name"`
		} `json:"author"`
		Identifier []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"identifier"`
	} `json:"bibjson"`
}

// Search searches for articles in DOAJ
func (s *DOAJService) Search(req models.SearchRequest) (models.SearchResponse, error) {
	resp := models.SearchResponse{
		Source:   "DOAJ",
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
	queryURL := fmt.Sprintf("%s/%s?page=%d&pageSize=%d",
		s.BaseURL,
		url.QueryEscape(req.Query),
		req.Page,
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

	var doajResp DOAJResponse
	if err := json.Unmarshal(body, &doajResp); err != nil {
		return resp, fmt.Errorf("failed to parse response: %w", err)
	}

	resp.Total = doajResp.Total

	// Convert to our model
	for _, article := range doajResp.Results {
		journal := models.Journal{
			Source:       "DOAJ",
			IsOpenAccess: true, // DOAJ is always open access
		}

		// Title
		journal.Title = article.Bibjson.Title

		// Authors
		for _, auth := range article.Bibjson.Author {
			if auth.Name != "" {
				journal.Authors = append(journal.Authors, models.Author{
					Name: auth.Name,
				})
			}
		}

		// Abstract
		journal.Abstract = article.Bibjson.Abstract

		// Year
		if article.Bibjson.Year != "" {
			fmt.Sscanf(article.Bibjson.Year, "%d", &journal.Year)
		}

		// Language
		if len(article.Bibjson.Language) > 0 {
			journal.Language = article.Bibjson.Language[0]
		}

		// Subjects
		for _, s := range article.Bibjson.Subject {
			if s.Term != "" {
				journal.Subjects = append(journal.Subjects, s.Term)
			}
		}

		// Links
		for _, link := range article.Bibjson.Link {
			if link.Type == "fulltext" {
				journal.URL = link.URL
				// Check if this is a PDF link based on content type
				if strings.Contains(link.ContentType, "pdf") {
					journal.PDFURL = link.URL
				}
			}
		}

		// DOI
		for _, id := range article.Bibjson.Identifier {
			if id.Type == "doi" {
				journal.DOI = id.ID
			}
		}

		// ID
		journal.ID = "doaj_" + journal.DOI
		if journal.DOI == "" {
			journal.ID = "doaj_" + url.QueryEscape(journal.Title)
		}

		resp.Results = append(resp.Results, journal)
	}

	return resp, nil
}

// GetDetail gets article detail by DOI
func (s *DOAJService) GetDetail(doi string) (models.Journal, error) {
	var journal models.Journal

	queryURL := fmt.Sprintf("%s/%s?page=1&pageSize=1", s.BaseURL, url.PathEscape(doi))

	httpResp, err := s.HTTPClient.Get(queryURL)
	if err != nil {
		return journal, fmt.Errorf("failed to execute request: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return journal, fmt.Errorf("failed to read response: %w", err)
	}

	var doajResp DOAJResponse
	if err := json.Unmarshal(body, &doajResp); err != nil {
		return journal, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(doajResp.Results) == 0 {
		return journal, fmt.Errorf("article not found")
	}

	article := doajResp.Results[0]

	// Title
	journal.Title = article.Bibjson.Title

	// Authors
	for _, auth := range article.Bibjson.Author {
		if auth.Name != "" {
			journal.Authors = append(journal.Authors, models.Author{
				Name: auth.Name,
			})
		}
	}

	// Abstract
	journal.Abstract = article.Bibjson.Abstract

	// Year
	if article.Bibjson.Year != "" {
		fmt.Sscanf(article.Bibjson.Year, "%d", &journal.Year)
	}

	// Language
	if len(article.Bibjson.Language) > 0 {
		journal.Language = article.Bibjson.Language[0]
	}

	// Subjects
	for _, s := range article.Bibjson.Subject {
		if s.Term != "" {
			journal.Subjects = append(journal.Subjects, s.Term)
		}
	}

	// Links
	for _, link := range article.Bibjson.Link {
		if link.Type == "fulltext" {
			journal.URL = link.URL
			// Check if this is a PDF link based on content type
			if strings.Contains(link.ContentType, "pdf") {
				journal.PDFURL = link.URL
			}
		}
	}

	// DOI - extract from the actual response
	for _, id := range article.Bibjson.Identifier {
		if id.Type == "doi" {
			journal.DOI = id.ID
			break
		}
	}
	journal.IsOpenAccess = true
	journal.Source = "DOAJ"
	journal.ID = "doaj_" + journal.DOI
	if journal.DOI == "" {
		journal.ID = "doaj_" + url.QueryEscape(journal.Title)
	}

	return journal, nil
}

// GetName returns the service name
func (s *DOAJService) GetName() string {
	return "DOAJ"
}

// IsAvailable checks if the service is available
func (s *DOAJService) IsAvailable() bool {
	resp, err := s.HTTPClient.Get(s.BaseURL + "/test?page=1&pageSize=1")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// CleanHTML removes HTML tags from a string (shared utility)
func CleanHTML(s string) string {
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
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}