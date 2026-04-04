# JURNALIN - Agregator Jurnal Ilmiah

Agregator jurnal ilmiah open-source yang mencari dari **6 sumber** sekaligus: CrossRef, arXiv, DOAJ, Semantic Scholar, **Garuda (Indonesia)**, dan **Google Scholar (scraping)**.

## 🚀 Quick Start

```bash
# Clone & run
go mod download
go run main.go

# Server berjalan di http://localhost:8080
```

## 📚 Sumber Jurnal

| Sumber | Tipe | Deskripsi |
|--------|------|-----------|
| **CrossRef** | API | 150M+ artikel akademik dengan DOI |
| **arXiv** | API | 2M+ preprint fisika, matematika, CS |
| **DOAJ** | API | 8M+ open access journals |
| **Semantic Scholar** | API | 200M+ paper dengan AI |
| **🇮🇩 Garuda** | API | Jurnal ilmiah Indonesia (Kemdikbud) |
| **Google Scholar** | Scraping | Crawling hasil Google Scholar |

## 🗄️ Database (Supabase)

### Setup Tables
Jalankan SQL ini di Supabase SQL Editor:

```sql
CREATE TABLE search_logs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    query TEXT NOT NULL,
    ip_address INET,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE bookmarks (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id TEXT,
    journal_id TEXT NOT NULL,
    title TEXT NOT NULL,
    source TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_search_logs_query ON search_logs(query);
CREATE INDEX idx_bookmarks_user_id ON bookmarks(user_id);
```

### Environment Variables
```
SUPABASE_URL=https://your-project.supabase.co
SUPABASE_KEY=your-anon-key
```

## 🔧 REST API

```bash
# Search
GET /api/v1/search?q=machine+learning

# Search by source
GET /api/v1/search?q=AI&source=garuda
GET /api/v1/search?q=AI&source=googlescholar

# Journal detail
GET /api/v1/journal/crossref_12345

# Available sources
GET /api/v1/sources
```

## 📁 Struktur Proyek

```
JURNALIN/
├── main.go                      # Entry point
├── .env                         # Environment variables
├── models/
│   └── journal.go               # Data models
├── services/
│   ├── aggregator.go            # Multi-source aggregator
│   ├── crossref.go              # CrossRef API
│   ├── arxiv.go                 # arXiv API
│   ├── doaj.go                  # DOAJ API
│   ├── semanticscholar.go       # Semantic Scholar API
│   ├── garuda.go                # 🇮🇩 Garuda API (Indonesia)
│   ├── googlescholar.go         # Google Scholar Scraping
│   └── database.go              # Supabase database
├── handlers/
│   └── handlers.go              # HTTP handlers
├── templates/
│   ├── layout.html              # DaisyUI layout
│   ├── index.html               # Home page
│   ├── search.html              # Search results
│   ├── detail.html              # Journal detail
│   ├── about.html               # About page
│   └── error.html               # Error page
└── supabase/
    └── functions/
        └── search-logger/       # Edge function for logging
            └── index.ts
```

## 🎨 UI/UX

- **DaisyUI** - Modern component framework
- **Tailwind CSS** - Utility-first CSS
- **Font Awesome** - Icon library
- **Responsive** - Mobile-friendly design
- **Dark mode ready**

## 🔥 Fitur

- ✅ **Multi-source search** - 6 sumber sekaligus
- ✅ **Scraping Google Scholar** - Web scraping untuk hasil tambahan
- ✅ **Jurnal Indonesia** - Garuda dari Kemdikbud
- ✅ **Deduplication** - Hapus duplikat otomatis
- ✅ **Smart sorting** - Open Access first, by citations, by year
- ✅ **Database logging** - Log pencarian ke Supabase
- ✅ **Bookmarks** - Simpan jurnal favorit
- ✅ **REST API** - API publik untuk integrasi
- ✅ **Edge Functions** - Supabase edge functions ready
- ✅ **Error handling** - Graceful fallback untuk sumber yang down

## 🛠️ Tech Stack

- **Go 1.21+** - Backend language
- **Gin** - HTTP framework
- **Supabase** - Database & Edge Functions
- **DaisyUI + Tailwind** - Frontend
- **Deno** - Edge Functions runtime

## 📝 License

MIT License - Free to use and modify.