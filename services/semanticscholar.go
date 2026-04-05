package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"jurnalinal/models"
)

// SemanticScholarService handles Semantic Scholar API requests
type SemanticScholarService struct {
	BaseURL    string
	HTTPClient *http.Client
	APIKey     string
}

// NewSemanticScholarService creates a new Semantic Scholar service
func NewSemanticScholarService() *SemanticScholarService {
	apiKey := os.Getenv("SEMANTIC_SCHOLAR_API_KEY")
	return &SemanticScholarService{
		BaseURL: "https://api.semanticscholar.org/graph/v1/paper/search",
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		APIKey: apiKey,
	}
}

// SemanticScholarResponse represents the Semantic Scholar API response structure
type SemanticScholarResponse struct {
	Total  int                    `json:"total"`
	Data   []SemanticScholarPaper `json:"data"`
}

// SemanticScholarPaper represents a Semantic Scholar paper
type SemanticScholarPaper struct {
	PaperID            string   `json:"paperId"`
	Title              string   `json:"title"`
	Abstract           string   `json:"abstract"`
	Year               int      `json:"year"`
	Authors            []struct {
		AuthorID string `json:"authorId"`
		Name     string `json:"name"`
	} `json:"authors"`
	ExternalIDs      map[string]interface{} `json:"externalIds"`
	URL              string   `json:"url"`
	Venue            string   `json:"venue"`
	CitationCount    int      `json:"citationCount"`
	InfluentialCitationCount int `json:"influentialCitationCount"`
	OpenAccessPdf    *struct {
		URL    string `json:"url"`
		Status string `json:"status"`
	} `json:"openAccessPdf"`
	FieldsOfStudy []string `json:"fieldsOfStudy"`
	IsOpenAccess  bool     `json:"isOpenAccess"`
}

// Search searches for papers in Semantic Scholar
func (s *SemanticScholarService) Search(req models.SearchRequest) (models.SearchResponse, error) {
	resp := models.SearchResponse{
		Source:   "Semantic Scholar",
		Page:     req.Page,
		PageSize: req.PageSize,
	}

	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}

	// Build query URL with fields
	fields := "title,abstract,year,authors,externalIds,url,venue,citationCount,influentialCitationCount,openAccessPdf,fieldsOfStudy,isOpenAccess"
	queryURL := fmt.Sprintf("%s?query=%s&limit=%d&offset=%d&fields=%s",
		s.BaseURL,
		url.QueryEscape(req.Query),
		req.PageSize,
		(req.Page-1)*req.PageSize,
		fields,
	)

	// Make HTTP request with retry for rate limits
	httpResp, err := s.doRequest(queryURL)
	if err != nil {
		return resp, err
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return resp, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for rate limit error
	if httpResp.StatusCode == http.StatusTooManyRequests {
		return resp, fmt.Errorf("Semantic Scholar rate limit exceeded. Try again later or add an API key")
	}

	var ssResp SemanticScholarResponse
	if err := json.Unmarshal(body, &ssResp); err != nil {
		return resp, fmt.Errorf("failed to parse response: %w", err)
	}

	resp.Total = ssResp.Total

	// Convert to our model
	for _, paper := range ssResp.Data {
		journal := models.Journal{
			Source: "Semantic Scholar",
		}

		// Title
		journal.Title = paper.Title

		// Authors
		for _, auth := range paper.Authors {
			if auth.Name != "" {
				journal.Authors = append(journal.Authors, models.Author{
					Name: auth.Name,
				})
			}
		}

		// Abstract
		journal.Abstract = CleanHTML(paper.Abstract)

		// Year
		journal.Year = paper.Year

		// DOI
		if doi, ok := paper.ExternalIDs["DOI"]; ok && doi != nil {
			journal.DOI = doi.(string)
		}

		// URL
		journal.URL = paper.URL
		if journal.URL == "" && journal.DOI != "" {
			journal.URL = fmt.Sprintf("https://doi.org/%s", journal.DOI)
		}

		// PDF URL
		if paper.OpenAccessPdf != nil {
			journal.PDFURL = paper.OpenAccessPdf.URL
			journal.IsOpenAccess = true
		}

		// Language (default to English)
		journal.Language = "en"

		// Subjects
		journal.Subjects = paper.FieldsOfStudy

		// Citations
		journal.Citations = paper.CitationCount

		// ID
		journal.ID = "ss_" + paper.PaperID

		resp.Results = append(resp.Results, journal)
	}

	return resp, nil
}

// GetDetail gets paper detail by Paper ID
func (s *SemanticScholarService) GetDetail(paperID string) (models.Journal, error) {
	var journal models.Journal

	fields := "title,abstract,year,authors,externalIds,url,venue,citationCount,influentialCitationCount,openAccessPdf,fieldsOfStudy,isOpenAccess"
	queryURL := fmt.Sprintf("https://api.semanticscholar.org/graph/v1/paper/%s?fields=%s", paperID, fields)

	httpResp, err := s.doRequest(queryURL)
	if err != nil {
		return journal, err
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return journal, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for rate limit error
	if httpResp.StatusCode == http.StatusTooManyRequests {
		return journal, fmt.Errorf("Semantic Scholar rate limit exceeded. Try again later or add an API key")
	}

	var paper SemanticScholarPaper
	if err := json.Unmarshal(body, &paper); err != nil {
		return journal, fmt.Errorf("failed to parse response: %w", err)
	}

	// Title
	journal.Title = paper.Title

	// Authors
	for _, auth := range paper.Authors {
		if auth.Name != "" {
			journal.Authors = append(journal.Authors, models.Author{
				Name: auth.Name,
			})
		}
	}

	// Abstract
	journal.Abstract = CleanHTML(paper.Abstract)

	// Year
	journal.Year = paper.Year

	// DOI
	if doi, ok := paper.ExternalIDs["DOI"]; ok && doi != nil {
		journal.DOI = doi.(string)
	}

	// URL
	journal.URL = paper.URL
	if journal.URL == "" && journal.DOI != "" {
		journal.URL = fmt.Sprintf("https://doi.org/%s", journal.DOI)
	}

	// PDF URL
	if paper.OpenAccessPdf != nil {
		journal.PDFURL = paper.OpenAccessPdf.URL
		journal.IsOpenAccess = true
	}

	// Language
	journal.Language = "en"

	// Subjects
	journal.Subjects = paper.FieldsOfStudy

	// Citations
	journal.Citations = paper.CitationCount

	// Source and ID
	journal.Source = "Semantic Scholar"
	journal.ID = "ss_" + paper.PaperID

	return journal, nil
}

// GetName returns the service name
func (s *SemanticScholarService) GetName() string {
	return "Semantic Scholar"
}

// IsAvailable checks if the service is available
func (s *SemanticScholarService) IsAvailable() bool {
	resp, err := s.HTTPClient.Get("https://api.semanticscholar.org/health/check")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// doRequest makes an HTTP request with the API key header if available
func (s *SemanticScholarService) doRequest(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add API key header if available
	if s.APIKey != "" {
		req.Header.Set("x-api-key", s.APIKey)
	}

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	return resp, nil
}

// GetAuthorNames extracts author names as a comma-separated string
func GetAuthorNames(authors []struct {
	AuthorID string `json:"authorId"`
	Name     string `json:"name"`
}) string {
	var names []string
	for _, a := range authors {
		if a.Name != "" {
			names = append(names, a.Name)
		}
	}
	return strings.Join(names, ", ")
}
