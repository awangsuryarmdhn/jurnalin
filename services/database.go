package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

// DatabaseService handles Supabase database operations
type DatabaseService struct {
	SupabaseURL string
	SupabaseKey string
	HTTPClient  *http.Client
	UseSupabase bool
}

// NewDatabaseService creates a new database service
func NewDatabaseService() *DatabaseService {
	// Load .env file
	godotenv.Load()

	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_KEY")

	// Remove "set " prefix if present
	if len(supabaseURL) > 4 && supabaseURL[:4] == "set " {
		supabaseURL = supabaseURL[4:]
	}
	if len(supabaseKey) > 4 && supabaseKey[:4] == "set " {
		supabaseKey = supabaseKey[4:]
	}

	useSupabase := supabaseURL != "" && supabaseKey != ""

	if useSupabase {
		log.Printf("✅ Supabase connected: %s", supabaseURL)
		// Test connection
		db := &DatabaseService{
			SupabaseURL: supabaseURL,
			SupabaseKey: supabaseKey,
			HTTPClient:  &http.Client{Timeout: 10 * time.Second},
			UseSupabase: true,
		}
		if err := db.TestConnection(); err != nil {
			log.Printf("⚠️  Supabase connection test failed: %v", err)
			log.Printf("💡 Make sure tables exist in Supabase SQL Editor")
		} else {
			log.Printf("✅ Supabase connection test passed!")
		}
		return db
	}

	log.Printf("⚠️  Supabase not configured, using local mode only")
	return &DatabaseService{
		SupabaseURL: supabaseURL,
		SupabaseKey: supabaseKey,
		HTTPClient:  &http.Client{Timeout: 10 * time.Second},
		UseSupabase: false,
	}
}

// TestConnection tests the Supabase connection
func (db *DatabaseService) TestConnection() error {
	if !db.UseSupabase {
		return fmt.Errorf("database not configured")
	}

	req, err := http.NewRequest("GET", db.getURL("search_logs")+"?select=id&limit=1", nil)
	if err != nil {
		return err
	}
	db.setHeaders(req)

	resp, err := db.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("connection test failed with status: %d", resp.StatusCode)
	}
	return nil
}

// SearchLog represents a search log entry
type SearchLog struct {
	ID        int64     `json:"id,omitempty"`
	Query     string    `json:"query"`
	IPAddress string    `json:"ip_address"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// Bookmark represents a bookmark entry
type Bookmark struct {
	ID        int64     `json:"id,omitempty"`
	UserID    string    `json:"user_id,omitempty"`
	JournalID string    `json:"journal_id"`
	Title     string    `json:"title"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// LogSearch logs a search query to Supabase
func (db *DatabaseService) LogSearch(query, ip string) {
	if !db.UseSupabase {
		return
	}

	searchLog := SearchLog{Query: query, IPAddress: ip}
	body, _ := json.Marshal(searchLog)

	req, err := http.NewRequest("POST", db.getURL("search_logs"), bytes.NewBuffer(body))
	if err != nil {
		log.Printf("❌ Failed to create search log request: %v", err)
		return
	}
	db.setHeaders(req)

	resp, err := db.HTTPClient.Do(req)
	if err != nil {
		log.Printf("❌ Failed to log search: %v", err)
		return
	}
	defer resp.Body.Close()
}

// SaveBookmark saves a bookmark to Supabase
func (db *DatabaseService) SaveBookmark(userID, journalID, title, source string) error {
	if !db.UseSupabase {
		return fmt.Errorf("database not configured")
	}

	bookmark := Bookmark{UserID: userID, JournalID: journalID, Title: title, Source: source}
	body, _ := json.Marshal(bookmark)

	req, err := http.NewRequest("POST", db.getURL("bookmarks"), bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	db.setHeaders(req)

	resp, err := db.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("save bookmark failed with status: %d", resp.StatusCode)
	}
	return nil
}

// GetBookmarks gets bookmarks for a user
func (db *DatabaseService) GetBookmarks(userID string) ([]Bookmark, error) {
	var bookmarks []Bookmark
	if !db.UseSupabase {
		return bookmarks, nil
	}

	req, err := http.NewRequest("GET", db.getURL("bookmarks")+"?user_id=eq."+userID+"&order=created_at.desc", nil)
	if err != nil {
		return bookmarks, err
	}
	db.setHeaders(req)

	resp, err := db.HTTPClient.Do(req)
	if err != nil {
		return bookmarks, err
	}
	defer resp.Body.Close()

	json.NewDecoder(resp.Body).Decode(&bookmarks)
	return bookmarks, nil
}

// GetPopularSearches gets popular search queries
func (db *DatabaseService) GetPopularSearches(limit int) ([]SearchLog, error) {
	var searches []SearchLog
	if !db.UseSupabase {
		return searches, nil
	}

	url := fmt.Sprintf("%s?select=query,created_at&order=created_at.desc&limit=%d",
		db.getURL("search_logs"), limit)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return searches, err
	}
	db.setHeaders(req)

	resp, err := db.HTTPClient.Do(req)
	if err != nil {
		return searches, err
	}
	defer resp.Body.Close()

	json.NewDecoder(resp.Body).Decode(&searches)
	return searches, nil
}

// GetSearchStats gets search statistics
func (db *DatabaseService) GetSearchStats() (map[string]interface{}, error) {
	stats := map[string]interface{}{"total_searches": 0, "unique_queries": 0}
	if !db.UseSupabase {
		return stats, nil
	}

	req, err := http.NewRequest("GET", db.getURL("search_logs")+"?select=id&limit=1", nil)
	if err != nil {
		return stats, err
	}
	db.setHeaders(req)
	req.Header.Set("Prefer", "count=exact")

	resp, err := db.HTTPClient.Do(req)
	if err != nil {
		return stats, err
	}
	defer resp.Body.Close()

	contentRange := resp.Header.Get("Content-Range")
	if contentRange != "" {
		var total int
		fmt.Sscanf(contentRange, "*/%d", &total)
		stats["total_searches"] = total
	}
	return stats, nil
}

// getURL builds the Supabase URL for a table
func (db *DatabaseService) getURL(table string) string {
	return fmt.Sprintf("%s/rest/v1/%s", db.SupabaseURL, table)
}

// setHeaders sets the required Supabase headers
func (db *DatabaseService) setHeaders(req *http.Request) {
	req.Header.Set("apikey", db.SupabaseKey)
	req.Header.Set("Authorization", "Bearer "+db.SupabaseKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")
}

// SetupSQL returns the SQL needed to create the Supabase tables
func SetupSQL() string {
	return `
-- Create search_logs table
CREATE TABLE IF NOT EXISTS search_logs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    query TEXT NOT NULL,
    ip_address INET,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Create bookmarks table
CREATE TABLE IF NOT EXISTS bookmarks (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id TEXT,
    journal_id TEXT NOT NULL,
    title TEXT NOT NULL,
    source TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_search_logs_query ON search_logs(query);
CREATE INDEX IF NOT EXISTS idx_search_logs_created_at ON search_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_bookmarks_user_id ON bookmarks(user_id);
CREATE INDEX IF NOT EXISTS idx_bookmarks_journal_id ON bookmarks(journal_id);

-- Enable Row Level Security
ALTER TABLE search_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE bookmarks ENABLE ROW LEVEL SECURITY;

-- Create policies
CREATE POLICY "Allow all on search_logs" ON search_logs FOR ALL USING (true);
CREATE POLICY "Allow all on bookmarks" ON bookmarks FOR ALL USING (true);
`
}