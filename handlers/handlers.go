package handlers

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"jurnalinal/models"
	"jurnalinal/services"
	"jurnalinal/templates"
)

// Handler holds dependencies for HTTP handlers
type Handler struct {
	Aggregator *services.AggregatorService
	Database   *services.DatabaseService
	LayoutTmpl *template.Template
}

// NewHandler creates a new handler
func NewHandler() *Handler {
	// Parse layout template from embedded FS
	layoutTmpl := template.Must(template.ParseFS(templates.FS, "layout.html"))

	return &Handler{
		Aggregator: services.NewAggregatorService(),
		Database:   services.NewDatabaseService(),
		LayoutTmpl: layoutTmpl,
	}
}

// renderTemplate renders a template with layout
func (h *Handler) renderTemplate(c *gin.Context, tmplName string, data gin.H) {
	// Parse the content template with FuncMap
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"mul": func(a, b int) int { return a * b },
		"div": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"mod": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a % b
		},
		"limit": func(s []models.Author, n int) []models.Author {
			if len(s) <= n {
				return s
			}
			return s[:n]
		},
		"limitStr": func(s []string, n int) []string {
			if len(s) <= n {
				return s
			}
			return s[:n]
		},
		"truncate": func(s string, n int) string {
			if len(s) <= n {
				return s
			}
			return s[:n] + "..."
		},
		"urlEncode": func(s string) string {
			return url.QueryEscape(s)
		},
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		"apaFormat": func(j models.Journal) string {
			return formatAPA(j)
		},
		"mlaFormat": func(j models.Journal) string {
			return formatMLA(j)
		},
		"bibtexFormat": func(j models.Journal) string {
			return formatBibTeX(j)
		},
		"slice": func(s string, start, end int) string {
			r := []rune(s)
			if start >= len(r) {
				return ""
			}
			if end > len(r) {
				end = len(r)
			}
			return string(r[start:end])
		},
	}
	contentTmpl := template.Must(template.New(tmplName).Funcs(funcMap).ParseFS(templates.FS, tmplName))

	var contentBuf bytes.Buffer
	if err := contentTmpl.Execute(&contentBuf, data); err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}

	data["Content"] = template.HTML(contentBuf.String())

	var buf bytes.Buffer
	if err := h.LayoutTmpl.Execute(&buf, data); err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}

	c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
}

// Index handles the home page
func (h *Handler) Index(c *gin.Context) {
	h.renderTemplate(c, "index.html", gin.H{
		"Title":       "JURNALIN - Agregator Jurnal Ilmiah",
		"Description": "Cari jurnal ilmiah dari 4 sumber sekaligus: CrossRef, arXiv, DOAJ, dan Semantic Scholar. Gratis & Open Access.",
		"NavPage": "home",
	})
}

// Search handles search requests
func (h *Handler) Search(c *gin.Context) {
	query := c.Query("q")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	yearFrom, _ := strconv.Atoi(c.Query("year_from"))
	yearTo, _ := strconv.Atoi(c.Query("year_to"))
	language := c.Query("language")
	source := c.Query("source")
	sortBy := c.DefaultQuery("sort", "relevance")

	if query == "" {
		h.renderTemplate(c, "index.html", gin.H{
			"Title": "JURNALIN - Agregator Jurnal Ilmiah",
			"Error": "Silakan masukkan kata kunci pencarian",
			"NavPage": "home",
		})
		return
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}

	// Log search to database
	go h.Database.LogSearch(query, c.ClientIP())

	req := models.SearchRequest{
		Query:    query,
		Page:     page,
		PageSize: pageSize,
		YearFrom: yearFrom,
		YearTo:   yearTo,
		Language: language,
		SortBy:   sortBy,
		Source:   source,
	}

	var results models.AggregatedResponse

	// Search across all sources
	results, err := h.Aggregator.Search(req)
	if err != nil {
		h.renderTemplate(c, "search.html", gin.H{
			"Title":   "Hasil Pencarian - JURNALIN",
			"Query":   query,
			"Error":   "Terjadi kesalahan saat mencari jurnal: " + err.Error(),
			"NavPage": "search",
		})
		return
	}

	// Deduplicate results
	results.Results = services.DeduplicateResults(results.Results)

	// Check if JSON response requested
	if c.GetHeader("Accept") == "application/json" || c.Query("format") == "json" {
		c.JSON(http.StatusOK, results)
		return
	}

	// Calculate page info
	totalPages := 1
	if results.Total > 0 && pageSize > 0 {
		totalPages = (results.Total + pageSize - 1) / pageSize
	}

	// Get suggestion if no results
	suggestion := ""
	if results.Total == 0 {
		suggestion = getSuggestion(query)
	}

	// HTML response
	h.renderTemplate(c, "search.html", gin.H{
		"Title":       fmt.Sprintf("Hasil \"%s\" - JURNALIN", query),
		"Description": fmt.Sprintf("Ditemukan %d hasil pencarian untuk \"%s\" dari berbagai database jurnal ilmiah.", results.Total, query),
		"Query":       query,
		"Results":     results.Results,
		"Total":       results.Total,
		"Page":        page,
		"PageSize":    pageSize,
		"TotalPages":  totalPages,
		"Sources":     results.Sources,
		"YearFrom":    yearFrom,
		"YearTo":      yearTo,
		"Language":    language,
		"Source":      source,
		"SortBy":      sortBy,
		"CurrentPage": "search",
		"Suggestion":  suggestion,
	})
}

// Detail handles journal detail requests
func (h *Handler) Detail(c *gin.Context) {
	id := strings.TrimPrefix(c.Param("id"), "/")

	if id == "" {
		h.renderTemplate(c, "error.html", gin.H{
			"Title": "Jurnal Tidak Ditemukan - JURNALIN",
			"Error": "ID jurnal tidak valid",
			"NavPage": "error",
		})
		return
	}

	journal, err := h.Aggregator.GetDetail(id)
	if err != nil {
		h.renderTemplate(c, "error.html", gin.H{
			"Title": "Jurnal Tidak Ditemukan - JURNALIN",
			"Error": "Jurnal tidak ditemukan: " + err.Error(),
			"NavPage": "error",
		})
		return
	}

	if journal.Title == "" {
		h.renderTemplate(c, "error.html", gin.H{
			"Title": "Jurnal Tidak Ditemukan - JURNALIN",
			"Error": "Jurnal tidak ditemukan",
			"NavPage": "error",
		})
		return
	}

	h.renderTemplate(c, "detail.html", gin.H{
		"Title":       journal.Title + " - JURNALIN",
		"Description": truncateStr(journal.Abstract, 160),
		"Journal":     journal,
		"NavPage": "detail",
	})
}

// About handles the about page
func (h *Handler) About(c *gin.Context) {
	h.renderTemplate(c, "about.html", gin.H{
		"Title":       "Tentang JURNALIN - Agregator Jurnal Ilmiah",
		"Description": "JURNALIN adalah platform agregator jurnal ilmiah gratis yang mengintegrasikan CrossRef, arXiv, DOAJ, dan Semantic Scholar.",
		"NavPage": "about",
	})
}

// APIDoc handles the API documentation page
func (h *Handler) APIDoc(c *gin.Context) {
	h.renderTemplate(c, "apidoc.html", gin.H{
		"Title":       "API Dokumentasi - JURNALIN",
		"Description": "Dokumentasi REST API JURNALIN untuk integrasi dengan aplikasi lain.",
		"NavPage": "api",
	})
}

// PDFProxy proxies PDF download from external sources
func (h *Handler) PDFProxy(c *gin.Context) {
	pdfURL := c.Query("url")
	if pdfURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL parameter required"})
		return
	}

	// Validate URL
	parsedURL, err := url.Parse(pdfURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid URL"})
		return
	}

	// Only allow known academic domains for security
	allowedDomains := []string{
		"arxiv.org", "doaj.org", "semanticscholar.org", "crossref.org",
		"doi.org", "ncbi.nlm.nih.gov", "pubmed.ncbi.nlm.nih.gov",
		"europepmc.org", "unpaywall.org", "core.ac.uk", "plos.org",
		"frontiersin.org", "mdpi.com", "springer.com", "nature.com",
		"biorxiv.org", "medrxiv.org", "hal.science", "zenodo.org",
		"researchgate.net",
	}

	host := strings.ToLower(parsedURL.Hostname())
	allowed := false
	for _, domain := range allowedDomains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			allowed = true
			break
		}
	}

	if !allowed {
		// Redirect to the URL directly if domain not in whitelist
		c.Redirect(http.StatusFound, pdfURL)
		return
	}

	// Fetch PDF
	client := &http.Client{
		Timeout: 60 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	req, err := http.NewRequest("GET", pdfURL, nil)
	if err != nil {
		c.Redirect(http.StatusFound, pdfURL)
		return
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; JURNALIN/1.0; +https://jurnalin.app)")
	req.Header.Set("Accept", "application/pdf,*/*")

	resp, err := client.Do(req)
	if err != nil {
		c.Redirect(http.StatusFound, pdfURL)
		return
	}
	defer resp.Body.Close()

	// Check if it's actually a PDF
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "pdf") && !strings.Contains(contentType, "octet-stream") {
		// Not a PDF, redirect to source
		c.Redirect(http.StatusFound, pdfURL)
		return
	}

	// Generate filename
	filename := c.Param("filename")
	if filename == "" || filename == "/" {
		filename = c.Query("title")
	}

	if filename == "" {
		filename = filepath.Base(parsedURL.Path)
	}

	// Sanitize filename
	filename = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == ' ' || r == '-' || r == '_' {
			return r
		}
		return -1
	}, filename)
	filename = strings.TrimSpace(filename)
	
	if filename == "" || filename == "pdf" || filename == "download" {
		filename = "Jurnal_Ilmiah"
	}
	
	filename = strings.ReplaceAll(filename, " ", "_")

	if !strings.HasSuffix(strings.ToLower(filename), ".pdf") {
		filename += ".pdf"
	}

	// Stream to client
	contentLengthStr := resp.Header.Get("Content-Length")
	var contentLength int64 = -1
	if contentLengthStr != "" {
		fmt.Sscanf(contentLengthStr, "%d", &contentLength)
	}

	extraHeaders := map[string]string{
		"Content-Disposition": fmt.Sprintf(`attachment; filename="%s"`, filename),
		"Cache-Control":       "no-cache",
	}

	c.DataFromReader(http.StatusOK, contentLength, "application/pdf", resp.Body, extraHeaders)
}

// APIv1Search handles API v1 search requests
func (h *Handler) APIv1Search(c *gin.Context) {
	query := c.Query("q")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	source := c.Query("source")
	yearFrom, _ := strconv.Atoi(c.Query("year_from"))
	yearTo, _ := strconv.Atoi(c.Query("year_to"))
	language := c.Query("language")

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query parameter 'q' is required"})
		return
	}

	req := models.SearchRequest{
		Query:    query,
		Page:     page,
		PageSize: pageSize,
		YearFrom: yearFrom,
		YearTo:   yearTo,
		Language: language,
	}

	// If specific source requested, use only that source
	if source != "" {
		var svc services.JournalService
		switch strings.ToLower(source) {
		case "crossref":
			svc = services.NewCrossRefService()
		case "arxiv":
			svc = services.NewArXivService()
		case "doaj":
			svc = services.NewDOAJService()
		case "semanticscholar":
			svc = services.NewSemanticScholarService()
		}

		if svc != nil {
			searchResp, err := svc.Search(req)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, searchResp)
			return
		}
	}

	results, err := h.Aggregator.Search(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Deduplicate
	results.Results = services.DeduplicateResults(results.Results)

	c.JSON(http.StatusOK, results)
}

// APIv1Detail handles API v1 detail requests
func (h *Handler) APIv1Detail(c *gin.Context) {
	id := strings.TrimPrefix(c.Param("id"), "/")

	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID parameter is required"})
		return
	}

	journal, err := h.Aggregator.GetDetail(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Journal not found"})
		return
	}

	if journal.Title == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Journal not found"})
		return
	}

	c.JSON(http.StatusOK, journal)
}

// APIv1Sources handles API v1 sources request
func (h *Handler) APIv1Sources(c *gin.Context) {
	sources := h.Aggregator.GetAvailableSources()
	c.JSON(http.StatusOK, gin.H{
		"sources": sources,
		"total":   len(sources),
		"version": "1.0",
		"docs":    "/api",
	})
}

// Helper
func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// formatAPA returns APA citation format for a journal
func formatAPA(j models.Journal) string {
	authors := ""
	for i, a := range j.Authors {
		if i >= 6 {
			authors += " et al."
			break
		}
		if i > 0 {
			authors += ", "
		}
		parts := strings.Fields(a.Name)
		if len(parts) > 1 {
			lastName := parts[len(parts)-1]
			initials := ""
			for _, p := range parts[:len(parts)-1] {
				if len(p) > 0 {
					initials += string([]rune(p)[0]) + "."
				}
			}
			authors += lastName + ", " + initials
		} else {
			authors += a.Name
		}
	}
	year := ""
	if j.Year > 0 {
		year = fmt.Sprintf(" (%d).", j.Year)
	} else {
		year = " (n.d.)."
	}
	doi := ""
	if j.DOI != "" {
		doi = " https://doi.org/" + j.DOI
	} else if j.URL != "" {
		doi = " " + j.URL
	}
	return fmt.Sprintf("%s%s %s.%s", authors, year, j.Title, doi)
}

// formatMLA returns MLA citation format
func formatMLA(j models.Journal) string {
	authors := ""
	if len(j.Authors) > 0 {
		a := j.Authors[0]
		parts := strings.Fields(a.Name)
		if len(parts) > 1 {
			authors = parts[len(parts)-1] + ", " + strings.Join(parts[:len(parts)-1], " ")
		} else {
			authors = a.Name
		}
		if len(j.Authors) == 2 {
			authors += ", and " + j.Authors[1].Name
		} else if len(j.Authors) > 2 {
			authors += ", et al."
		}
	}
	year := ""
	if j.Year > 0 {
		year = fmt.Sprintf(", %d", j.Year)
	}
	source := ""
	if j.Source != "" {
		source = ". " + j.Source
	}
	doi := ""
	if j.DOI != "" {
		doi = ". doi:" + j.DOI
	}
	return fmt.Sprintf("%s. \"%s\"%s%s%s.", authors, j.Title, source, year, doi)
}

// formatBibTeX returns BibTeX format
func formatBibTeX(j models.Journal) string {
	key := "jurnal2024"
	if len(j.Authors) > 0 && j.Year > 0 {
		parts := strings.Fields(j.Authors[0].Name)
		lastName := parts[0]
		if len(parts) > 1 {
			lastName = parts[len(parts)-1]
		}
		lastName = strings.ToLower(strings.ReplaceAll(lastName, " ", ""))
		key = fmt.Sprintf("%s%d", lastName, j.Year)
	}

	authorStr := ""
	for i, a := range j.Authors {
		if i > 0 {
			authorStr += " and "
		}
		authorStr += a.Name
	}

	result := fmt.Sprintf("@article{%s,\n", key)
	if authorStr != "" {
		result += fmt.Sprintf("  author    = {%s},\n", authorStr)
	}
	if j.Title != "" {
		result += fmt.Sprintf("  title     = {%s},\n", j.Title)
	}
	if j.Year > 0 {
		result += fmt.Sprintf("  year      = {%d},\n", j.Year)
	}
	if j.Source != "" {
		result += fmt.Sprintf("  journal   = {%s},\n", j.Source)
	}
	if j.DOI != "" {
		result += fmt.Sprintf("  doi       = {%s},\n", j.DOI)
	}
	if j.URL != "" {
		result += fmt.Sprintf("  url       = {%s},\n", j.URL)
	}
	result += "}"
	return result
}

// Sitemap handles sitemap.xml requests
func (h *Handler) Sitemap(c *gin.Context) {
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)

	sitemap := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>%s/</loc>
    <changefreq>daily</changefreq>
    <priority>1.0</priority>
  </url>
  <url>
    <loc>%s/about</loc>
    <changefreq>monthly</changefreq>
    <priority>0.8</priority>
  </url>
  <url>
    <loc>%s/api</loc>
    <changefreq>monthly</changefreq>
    <priority>0.5</priority>
  </url>
</urlset>`, baseURL, baseURL, baseURL)
	c.Header("Content-Type", "application/xml")
	c.String(http.StatusOK, sitemap)
}

// Manifest handles manifest.json requests
func (h *Handler) Manifest(c *gin.Context) {
	manifest := gin.H{
		"name":             "JURNALIN",
		"short_name":        "JURNALIN",
		"start_url":         "/",
		"display":           "standalone",
		"background_color":  "#ffffff",
		"theme_color":       "#10b981",
		"description":       "Agregator Jurnal Ilmiah Indonesia",
		"icons": []gin.H{
			{
				"src":   "/static/images/favicon.png",
				"sizes": "192x192",
				"type":  "image/png",
			},
			{
				"src":   "/static/images/favicon.png",
				"sizes": "512x512",
				"type":  "image/png",
			},
		},
	}
	c.JSON(http.StatusOK, manifest)
}

// getSuggestion returns a corrected query for common typos
func getSuggestion(query string) string {
	commonTypos := map[string]string{
		"machine learnign":    "machine learning",
		"machine lerning":     "machine learning",
		"artifical":           "artificial",
		"intelegence":         "intelligence",
		"deep learnign":       "deep learning",
		"deep lerning":        "deep learning",
		"blockchain":          "block chain",
		"block chain":         "blockchain",
		"pendidikan":          "education",
		"edukasi":             "education",
		"komputer":            "computer",
		"teknologi":           "technology",
		"sains":               "science",
		"aritificial":         "artificial",
		"inteligence":         "intelligence",
		"artificial intel":    "artificial intelligence",
		"data scince":         "data science",
		"data sience":         "data science",
		"netrwork":            "network",
		"cyber securty":       "cyber security",
		"securty":             "security",
		"cloud computig":      "cloud computing",
		"big dta":             "big data",
		"internet of thing":   "internet of things",
	}

	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	for typo, correction := range commonTypos {
		if strings.Contains(lowerQuery, typo) {
			return strings.ReplaceAll(lowerQuery, typo, correction)
		}
	}

	return ""
}

