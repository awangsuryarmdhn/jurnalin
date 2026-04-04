package services

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"jurnalinal/models"
)

// JournalService interface for all journal services
type JournalService interface {
	Search(req models.SearchRequest) (models.SearchResponse, error)
	GetName() string
	IsAvailable() bool
}

// AggregatorService aggregates results from multiple sources
type AggregatorService struct {
	Services []JournalService
	Timeout  time.Duration
}

// NewAggregatorService creates a new aggregator service
func NewAggregatorService() *AggregatorService {
	return &AggregatorService{
		Services: []JournalService{
			NewCrossRefService(),
			NewArXivService(),
			NewDOAJService(),
			NewSemanticScholarService(),
		},
		Timeout: 30 * time.Second,
	}
}

// Search searches across all available sources and aggregates results
func (a *AggregatorService) Search(req models.SearchRequest) (models.AggregatedResponse, error) {
	resp := models.AggregatedResponse{
		Page:     req.Page,
		PageSize: req.PageSize,
		Query:    req.Query,
	}

	if req.PageSize < 1 {
		req.PageSize = 20
	}

	type searchResult struct {
		Response models.SearchResponse
		Error    error
		Source   string
	}

	resultChan := make(chan searchResult, len(a.Services))
	var wg sync.WaitGroup

	ctx, cancel := context.WithTimeout(context.Background(), a.Timeout)
	defer cancel()

	for _, service := range a.Services {
		wg.Add(1)
		go func(svc JournalService) {
			defer wg.Done()

			if !svc.IsAvailable() {
				resultChan <- searchResult{Source: svc.GetName()}
				return
			}

			done := make(chan searchResult, 1)
			go func() {
				searchResp, err := svc.Search(req)
				done <- searchResult{Response: searchResp, Error: err, Source: svc.GetName()}
			}()

			select {
			case result := <-done:
				resultChan <- result
			case <-ctx.Done():
				resultChan <- searchResult{Source: svc.GetName(), Error: ctx.Err()}
			}
		}(service)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var sources []string
	var errs []string
	for result := range resultChan {
		if result.Error != nil {
			errs = append(errs, fmt.Sprintf("%s (%v)", result.Source, result.Error))
			continue
		}
		if result.Response.Total > 0 || len(result.Response.Results) > 0 {
			resp.Results = append(resp.Results, result.Response.Results...)
			resp.Total += result.Response.Total
			sources = append(sources, result.Source)
		}
	}

	resp.Sources = sources

	if len(resp.Results) == 0 && len(errs) > 0 {
		return resp, fmt.Errorf("Semua pencarian gagal atau timeout. Detail: %s", strings.Join(errs, ", "))
	}

	currentYear := time.Now().Year()

	// 1. Calculate Score for each result
	for i := range resp.Results {
		var score float64 = 0
		
		// Weight 1: Open Access (High priority for accessibility)
		if resp.Results[i].IsOpenAccess {
			score += 50
		}
		
		// Weight 2: Citations (Quality indicator)
		citations := float64(resp.Results[i].Citations)
		if citations > 1000 { citations = 1000 }
		score += (citations / 1000.0) * 30
		
		// Weight 3: Recency (Freshness)
		if resp.Results[i].Year > 0 {
			age := currentYear - resp.Results[i].Year
			if age < 0 { age = 0 }
			if age < 20 {
				score += (1.0 - float64(age)/20.0) * 20
			}
		}
		
		resp.Results[i].Score = score
	}

	// Sort by Score descending
	sort.SliceStable(resp.Results, func(i, j int) bool {
		return resp.Results[i].Score > resp.Results[j].Score
	})

	if len(resp.Results) > req.PageSize {
		resp.Results = resp.Results[:req.PageSize]
	}

	return resp, nil
}

// GetDetail gets journal detail by ID
func (a *AggregatorService) GetDetail(id string) (models.Journal, error) {
	var journal models.Journal

	parts := strings.SplitN(id, "_", 2)
	if len(parts) < 2 {
		return journal, nil
	}

	source := parts[0]
	sourceID := parts[1]

	switch source {
	case "crossref":
		return NewCrossRefService().GetDetail(sourceID)
	case "arxiv":
		return NewArXivService().GetDetail(sourceID)
	case "doaj":
		return NewDOAJService().GetDetail(sourceID)
	case "ss":
		return NewSemanticScholarService().GetDetail(sourceID)
	}

	return journal, nil
}

// GetAvailableSources returns list of available sources
func (a *AggregatorService) GetAvailableSources() []string {
	var available []string
	for _, svc := range a.Services {
		if svc.IsAvailable() {
			available = append(available, svc.GetName())
		}
	}
	return available
}

// DeduplicateResults removes duplicate journals with better title normalization
func DeduplicateResults(results []models.Journal) []models.Journal {
	seen := make(map[string]bool)
	var deduped []models.Journal

	for _, journal := range results {
		// Priority 1: DOI normalization
		key := strings.ToLower(strings.TrimSpace(journal.DOI))
		
		// Priority 2: Title normalization (Remove spaces, special chars, lowercase)
		if key == "" {
			t := strings.ToLower(journal.Title)
			t = strings.Map(func(r rune) rune {
				if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
					return r
				}
				return -1
			}, t)
			key = t
		}
		
		if key != "" && !seen[key] {
			seen[key] = true
			deduped = append(deduped, journal)
		}
	}
	return deduped
}
