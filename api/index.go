package handler

import (
	"net/http"
	"jurnalinal/handlers"
	"jurnalinal/middlewares"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

var app *gin.Engine

func init() {
	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)

	// Create Gin router
	r := gin.Default()

	// Rate Limiter
	limiter := middlewares.NewRateLimiter(rate.Limit(5), 15)
	r.Use(limiter.IPRateLimitMiddleware())

	// Create handler
	h := handlers.NewHandler()

	// NOTE: For Vercel, templates and static files should be handled carefully.
	// Since we are using standard gin.HTMLRender, we'll rely on the project structure.
	// Vercel copies the entire repo to the function, so relative paths usually work.
	
	// Routes
	r.GET("/", h.Index)
	r.GET("/search", h.Search)
	r.GET("/journal/*id", h.Detail)
	r.GET("/about", h.About)
	r.GET("/api", h.APIDoc)
	r.GET("/sitemap.xml", h.Sitemap)
	r.GET("/manifest.json", h.Manifest)
	r.GET("/pdf/proxy", h.PDFProxy)
	r.GET("/pdf/proxy/:filename", h.PDFProxy)

	api := r.Group("/api/v1")
	{
		api.GET("/search", h.APIv1Search)
		api.GET("/journal/*id", h.APIv1Detail)
		api.GET("/sources", h.APIv1Sources)
	}

	app = r
}

// Handler is the entry point for Vercel
func Handler(w http.ResponseWriter, r *http.Request) {
	app.ServeHTTP(w, r)
}
