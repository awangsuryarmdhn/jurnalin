package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"jurnalinal/handlers"
	"jurnalinal/middlewares"
)

func main() {
	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)

	// Create Gin router
	r := gin.Default()

	// Rate Limiter: 5 requests per second, burst up to 15 per IP (cukup untuk mahasiswa/riset normal)
	limiter := middlewares.NewRateLimiter(rate.Limit(5), 15)
	r.Use(limiter.IPRateLimitMiddleware())

	// Create handler
	h := handlers.NewHandler()

	// Static files
	r.Static("/static", "./static")

	// Web routes
	r.GET("/", h.Index)
	r.GET("/search", h.Search)
	r.GET("/journal/*id", h.Detail)
	r.GET("/about", h.About)
	r.GET("/api", h.APIDoc)
	r.GET("/sitemap.xml", h.Sitemap)
	r.GET("/manifest.json", h.Manifest)

	// PDF Proxy - download PDF langsung dari server
	r.GET("/pdf/proxy", h.PDFProxy)
	r.GET("/pdf/proxy/:filename", h.PDFProxy)

	// API v1 routes
	api := r.Group("/api/v1")
	{
		api.GET("/search", h.APIv1Search)
		api.GET("/journal/*id", h.APIv1Detail)
		api.GET("/sources", h.APIv1Sources)
	}

	// Custom 404 handler
	r.NoRoute(func(c *gin.Context) {
		c.Data(http.StatusNotFound, "text/html; charset=utf-8", []byte(`
			<!DOCTYPE html>
			<html lang="id" data-theme="jurnalin">
			<head>
				<meta charset="UTF-8">
				<meta name="viewport" content="width=device-width, initial-scale=1.0">
				<title>404 - JURNALIN</title>
				<link href="https://cdn.jsdelivr.net/npm/daisyui@4.12.14/dist/full.min.css" rel="stylesheet" />
				<script src="https://cdn.tailwindcss.com"></script>
				<link rel="preconnect" href="https://fonts.googleapis.com">
				<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;600;700;900&display=swap" rel="stylesheet">
				<style>* { font-family: 'Inter', sans-serif; }</style>
			</head>
			<body class="min-h-screen bg-gradient-to-br from-slate-900 via-indigo-950 to-slate-900 flex items-center justify-center">
				<div class="text-center px-4">
					<div class="text-9xl font-black text-indigo-500 opacity-30 mb-4">404</div>
					<h1 class="text-4xl font-bold text-white mb-4">Halaman Tidak Ditemukan</h1>
					<p class="text-slate-400 mb-8 max-w-md mx-auto">Halaman yang Anda cari tidak ada atau sudah dipindahkan.</p>
					<a href="/" class="inline-flex items-center gap-2 bg-indigo-600 hover:bg-indigo-500 text-white font-semibold px-6 py-3 rounded-xl transition">
						<svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" /></svg>
						Kembali ke Beranda
					</a>
				</div>
			</body>
			</html>
		`))
	})

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🚀 JURNALIN - Agregator Jurnal Ilmiah")
	log.Printf("🌐 Server berjalan di http://localhost:%s", port)
	log.Printf("📚 Sumber: CrossRef, arXiv, DOAJ, Semantic Scholar")
	log.Printf("🔧 API tersedia di http://localhost:%s/api/v1", port)
	log.Printf("📥 PDF Proxy: http://localhost:%s/pdf/proxy?url=<url>", port)

	if err := r.Run(":" + port); err != nil {
		log.Fatalf("❌ Gagal menjalankan server: %v", err)
	}
}