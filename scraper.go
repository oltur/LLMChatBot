package main

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

type WebScraper struct {
	client              *http.Client
	cache               map[string]WebsiteContent
	pdfExtractor        *PDFExtractor
	pdfCache            map[string]*PDFContent
	fileParser          *FileParser
	fileCache           map[string]*FileContent
	allowedUrlPatterns  []string
	scrapedUrls         []ScrapedUrl
	enableInternalLinks bool
	refreshContent      bool
	cacheDir            string
	minTextLength       int
	maxContentLength    int
	maxScrapingDepth    int
	visitedUrls         map[string]bool
	maxPagesPerSession  int
	scrapedPagesCount   int
	ollamaService       *OllamaService
	cacheDuration       time.Duration
	memoryCacheDuration time.Duration
}

type ScrapedUrl struct {
	URL         string
	Type        string // "main", "linked", "first_level", "pdf", "file"
	Title       string
	Success     bool
	Error       string
	ScrapedAt   time.Time
	Relevance   int
	ContentType string
}

type WebsiteContent struct {
	Title         string
	Description   string
	Links         []Link
	Text          string
	PDFContent    map[string]*PDFContent
	FileContent   map[string]*FileContent
	LinkedContent map[string]*LinkedPageContent
	Metadata      map[string]string
	LastUpdated   time.Time
	ContentHash   string // SHA256 hash of raw page content
}

type LinkedPageContent struct {
	URL             string
	Title           string
	Text            string
	Description     string
	Keywords        []string
	Relevance       int    // 1-10 relevance score
	ContentType     string // "professional", "blog", "project", "general"
	FirstLevelLinks []FirstLevelLink
	LastUpdated     time.Time
	ContentHash     string // SHA256 hash of raw page content
}

type FirstLevelLink struct {
	URL         string
	Title       string
	Text        string
	Description string
	Relevance   int
	LastUpdated time.Time
}

type Link struct {
	URL   string
	Title string
	Type  string
}

func NewWebScraper(ollamaService *OllamaService) *WebScraper {
	// Parse allowed URL patterns from environment variable
	allowedPatternsStr := os.Getenv("ALLOWED_SCRAPING_URL_PATTERNS")
	var allowedUrlPatterns []string

	if allowedPatternsStr != "" {
		// Split by comma and trim whitespace
		patterns := strings.Split(allowedPatternsStr, ",")
		for _, pattern := range patterns {
			trimmed := strings.TrimSpace(pattern)
			if trimmed != "" {
				allowedUrlPatterns = append(allowedUrlPatterns, strings.ToLower(trimmed))
			}
		}
	}

	// Check if internal link processing is enabled
	enableInternal := strings.ToLower(os.Getenv("ENABLE_INTERNAL_LINK_SCRAPING")) == "true"

	// Check if content refresh is enabled (default: false for performance)
	refreshContent := strings.ToLower(os.Getenv("REFRESH_CONTENT")) == "true"

	// Parse minimum text length (default: 10)
	minTextLength := 10
	if minTextLengthStr := os.Getenv("MIN_TEXT_LENGTH"); minTextLengthStr != "" {
		if parsed, err := strconv.Atoi(minTextLengthStr); err == nil && parsed > 0 {
			minTextLength = parsed
		}
	}

	// Parse maximum text length (default: 10000)
	maxContentLength := 10000
	if maxContentLengthStr := os.Getenv("MAX_CONTENT_LENGTH"); maxContentLengthStr != "" {
		if parsed, err := strconv.Atoi(maxContentLengthStr); err == nil && parsed > minTextLength {
			maxContentLength = parsed
		}
	}

	// Parse maximum scraping depth (default: 2)
	maxScrapingDepth := 2
	if maxDepthStr := os.Getenv("MAX_SCRAPING_DEPTH"); maxDepthStr != "" {
		if parsed, err := strconv.Atoi(maxDepthStr); err == nil && parsed >= 1 && parsed <= 10 {
			maxScrapingDepth = parsed
		}
	}

	// Parse maximum pages per session (default: 100)
	maxPagesPerSession := 100
	if maxPagesStr := os.Getenv("MAX_PAGES_PER_SESSION"); maxPagesStr != "" {
		if parsed, err := strconv.Atoi(maxPagesStr); err == nil && parsed > 0 {
			maxPagesPerSession = parsed
		}
	}

	// Parse cache duration (default: 24 hours)
	cacheDuration := 24 * time.Hour
	if cacheDurationStr := os.Getenv("CACHE_DURATION_HOURS"); cacheDurationStr != "" {
		if parsed, err := strconv.Atoi(cacheDurationStr); err == nil && parsed > 0 {
			cacheDuration = time.Duration(parsed) * time.Hour
		}
	}

	// Memory cache duration (use shorter duration, typically 1 hour or 1/24 of cache duration)
	memoryCacheDuration := 1 * time.Hour
	if cacheDuration < 24*time.Hour {
		memoryCacheDuration = cacheDuration / 24
		if memoryCacheDuration < 15*time.Minute {
			memoryCacheDuration = 15 * time.Minute
		}
	}

	// Create cache directory
	cacheDir := "scraped_content"
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		fmt.Printf("Warning: Could not create cache directory: %v\n", err)
	}

	return &WebScraper{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		cache:               make(map[string]WebsiteContent),
		pdfExtractor:        NewPDFExtractor(),
		pdfCache:            make(map[string]*PDFContent),
		fileParser:          NewFileParser(),
		fileCache:           make(map[string]*FileContent),
		allowedUrlPatterns:  allowedUrlPatterns,
		scrapedUrls:         make([]ScrapedUrl, 0),
		enableInternalLinks: enableInternal,
		refreshContent:      refreshContent,
		cacheDir:            cacheDir,
		minTextLength:       minTextLength,
		maxContentLength:    maxContentLength,
		maxScrapingDepth:    maxScrapingDepth,
		visitedUrls:         make(map[string]bool),
		maxPagesPerSession:  maxPagesPerSession,
		scrapedPagesCount:   0,
		ollamaService:       ollamaService,
		cacheDuration:       cacheDuration,
		memoryCacheDuration: memoryCacheDuration,
	}
}

// generateSafeDirectoryName creates a safe directory name from a URL
func (w *WebScraper) generateSafeDirectoryName(targetUrl string) string {
	// Parse URL to get domain
	parsedURL, err := url.Parse(targetUrl)
	if err != nil {
		// Fallback to MD5 hash if URL parsing fails
		hasher := md5.New()
		hasher.Write([]byte(targetUrl))
		return hex.EncodeToString(hasher.Sum(nil))
	}

	// Create a safe directory name: domain + path hash
	domain := parsedURL.Host
	path := parsedURL.Path
	query := parsedURL.RawQuery

	// Remove common prefixes
	domain = strings.TrimPrefix(domain, "www.")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimPrefix(domain, "https://")

	// Replace unsafe characters in domain
	domainSafe := regexp.MustCompile(`[^a-zA-Z0-9.-]`).ReplaceAllString(domain, "_")

	// Create hash of path + query for uniqueness
	fullPath := path
	if query != "" {
		fullPath += "?" + query
	}

	hasher := md5.New()
	hasher.Write([]byte(fullPath))
	pathHash := hex.EncodeToString(hasher.Sum(nil))[:8] // First 8 characters

	if fullPath == "/" || fullPath == "" {
		return domainSafe
	}

	return domainSafe + "_" + pathHash
}

// getContentFilePath returns the file path for storing content
func (w *WebScraper) getContentFilePath(targetUrl string) string {
	dirName := w.generateSafeDirectoryName(targetUrl)
	dirPath := filepath.Join(w.cacheDir, dirName)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		fmt.Printf("Warning: Could not create directory %s: %v\n", dirPath, err)
	}

	return filepath.Join(dirPath, "content.json")
}

// saveContentToDisk saves website content to disk
func (w *WebScraper) saveContentToDisk(targetUrl string, content *WebsiteContent) error {
	filePath := w.getContentFilePath(targetUrl)

	// Create a wrapper structure to include the URL
	wrapper := struct {
		URL     string          `json:"url"`
		SavedAt time.Time       `json:"saved_at"`
		Content *WebsiteContent `json:"content"`
	}{
		URL:     targetUrl,
		SavedAt: time.Now(),
		Content: content,
	}

	data, err := json.MarshalIndent(wrapper, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal content: %v", err)
	}

	if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	fmt.Printf("Content saved to: %s\n", filePath)
	return nil
}

// loadContentFromDisk loads website content from disk
func (w *WebScraper) loadContentFromDisk(targetUrl string) (*WebsiteContent, error) {
	filePath := w.getContentFilePath(targetUrl)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("content file does not exist")
	}

	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	wrapper := struct {
		URL     string          `json:"url"`
		SavedAt time.Time       `json:"saved_at"`
		Content *WebsiteContent `json:"content"`
	}{}

	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to unmarshal content: %v", err)
	}

	fmt.Printf("Content loaded from: %s (saved at %s)\n", filePath, wrapper.SavedAt.Format("2006-01-02 15:04:05"))
	return wrapper.Content, nil
}

// normalizeURL normalizes a URL for consistent loop detection
func (w *WebScraper) normalizeURL(targetUrl string) string {
	// Parse URL to normalize it
	parsedURL, err := url.Parse(strings.ToLower(targetUrl))
	if err != nil {
		return strings.ToLower(targetUrl) // fallback
	}

	// Remove common query parameters that don't affect content
	query := parsedURL.Query()
	query.Del("utm_source")
	query.Del("utm_medium")
	query.Del("utm_campaign")
	query.Del("utm_term")
	query.Del("utm_content")
	query.Del("ref")
	query.Del("source")
	parsedURL.RawQuery = query.Encode()

	// Remove fragment
	parsedURL.Fragment = ""

	// Remove trailing slash from path
	if len(parsedURL.Path) > 1 && strings.HasSuffix(parsedURL.Path, "/") {
		parsedURL.Path = strings.TrimSuffix(parsedURL.Path, "/")
	}

	return parsedURL.String()
}

// isURLVisited checks if a URL has been visited (with normalization)
func (w *WebScraper) isURLVisited(targetUrl string) bool {
	normalizedUrl := w.normalizeURL(targetUrl)
	return w.visitedUrls[normalizedUrl]
}

// markURLVisited marks a URL as visited (with normalization)
func (w *WebScraper) markURLVisited(targetUrl string) {
	normalizedUrl := w.normalizeURL(targetUrl)
	w.visitedUrls[normalizedUrl] = true
}

// canScrapeMore checks if we can scrape more pages
func (w *WebScraper) canScrapeMore() bool {
	return w.scrapedPagesCount < w.maxPagesPerSession
}

// calculateContentHash generates SHA256 hash of raw HTML content
func (w *WebScraper) calculateContentHash(htmlContent string) string {
	hasher := sha256.New()
	hasher.Write([]byte(htmlContent))
	return hex.EncodeToString(hasher.Sum(nil))
}

// findContentByHash searches for existing content with the same hash
func (w *WebScraper) findContentByHash(contentHash string) (*WebsiteContent, error) {
	// Search through all cached content files
	entries, err := os.ReadDir(w.cacheDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			contentFile := filepath.Join(w.cacheDir, entry.Name(), "content.json")
			if data, err := ioutil.ReadFile(contentFile); err == nil {
				wrapper := struct {
					URL     string          `json:"url"`
					SavedAt time.Time       `json:"saved_at"`
					Content *WebsiteContent `json:"content"`
				}{}

				if err := json.Unmarshal(data, &wrapper); err == nil {
					if wrapper.Content != nil && wrapper.Content.ContentHash == contentHash {
						fmt.Printf("Found existing content with matching hash: %s (original URL: %s)\n",
							contentHash[:8], wrapper.URL)
						return wrapper.Content, nil
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("no content found with hash %s", contentHash)
}

func (w *WebScraper) isUrlAllowed(targetUrl string) bool {
	// If no allowed URL patterns are configured, allow all URLs
	if len(w.allowedUrlPatterns) == 0 {
		return true
	}

	// Normalize the URL for consistent matching
	normalizedUrl := strings.ToLower(targetUrl)

	// Check if URL matches any of the allowed patterns
	for _, pattern := range w.allowedUrlPatterns {
		if strings.Contains(normalizedUrl, pattern) {
			return true
		}
	}

	return false
}

func (w *WebScraper) recordScrapedUrl(url, urlType, title string, success bool, err error, relevance int, contentType string) {
	scrapedUrl := ScrapedUrl{
		URL:         url,
		Type:        urlType,
		Title:       title,
		Success:     success,
		ScrapedAt:   time.Now(),
		Relevance:   relevance,
		ContentType: contentType,
	}

	if err != nil {
		scrapedUrl.Error = err.Error()
	}

	w.scrapedUrls = append(w.scrapedUrls, scrapedUrl)
}

func (w *WebScraper) GetScrapedUrls() []ScrapedUrl {
	return w.scrapedUrls
}

func (w *WebScraper) ClearScrapedUrls() {
	w.scrapedUrls = make([]ScrapedUrl, 0)
	// Also reset visited URLs and page count for new session
	w.visitedUrls = make(map[string]bool)
	w.scrapedPagesCount = 0
}

func (w *WebScraper) PrintScrapedUrls() {
	fmt.Printf("\n=== SCRAPING SUMMARY ===\n")
	fmt.Printf("Total URLs processed: %d\n", len(w.scrapedUrls))

	// Count by type and status
	typeCount := make(map[string]int)
	successCount := 0
	failureCount := 0

	for _, scraped := range w.scrapedUrls {
		typeCount[scraped.Type]++
		if scraped.Success {
			successCount++
		} else {
			failureCount++
		}
	}

	fmt.Printf("Successful: %d, Failed: %d\n", successCount, failureCount)
	fmt.Printf("By type: ")
	for urlType, count := range typeCount {
		fmt.Printf("%s: %d, ", urlType, count)
	}
	fmt.Printf("\n\n")

	// Print detailed list
	fmt.Printf("Detailed scraping log:\n")
	for i, scraped := range w.scrapedUrls {
		status := "✓"
		if !scraped.Success {
			status = "✗"
		}

		title := scraped.Title
		if title == "" {
			title = "(no title)"
		}
		if len(title) > 50 {
			title = title[:50] + "..."
		}

		fmt.Printf("%d. %s [%s] %s - %s", i+1, status, scraped.Type, scraped.URL, title)
		if scraped.Relevance > 0 {
			fmt.Printf(" (relevance: %d)", scraped.Relevance)
		}
		if scraped.ContentType != "" {
			fmt.Printf(" [%s]", scraped.ContentType)
		}
		if !scraped.Success && scraped.Error != "" {
			fmt.Printf(" - Error: %s", scraped.Error)
		}
		fmt.Printf("\n")
	}
	fmt.Printf("========================\n\n")
}

func (w *WebScraper) ScrapeWebsite(targetUrl string) (*WebsiteContent, error) {
	return w.scrapeWebsiteWithDepth(targetUrl, 0)
}

// Common page scraping function that both main and linked page scrapers can use
func (w *WebScraper) scrapePage(targetUrl string, depth int, urlType string, useCache bool) (*goquery.Document, string, string, error) {
	// Check depth limit and page limit
	if depth >= w.maxScrapingDepth || !w.canScrapeMore() {
		return nil, "", "", fmt.Errorf("scraping limits reached: depth=%d, pages=%d", depth, w.scrapedPagesCount)
	}

	// Check if URL already visited (for linked pages)
	if urlType == "linked" && w.isURLVisited(targetUrl) {
		return nil, "", "", fmt.Errorf("URL already visited: %s", targetUrl)
	}

	// Check if the URL is allowed to be scraped
	if !w.isUrlAllowed(targetUrl) {
		err := fmt.Errorf("URL not allowed for scraping: %s", targetUrl)
		w.recordScrapedUrl(targetUrl, urlType, "", false, err, 0, "")
		return nil, "", "", err
	}

	// Mark URL as visited and increment counter for linked pages
	if urlType == "linked" {
		w.markURLVisited(targetUrl)
		w.scrapedPagesCount++
		log.Printf("Scraping linked page (depth %d): %s\n", depth, targetUrl)
	}

	var client *http.Client
	if urlType == "main" {
		client = w.client
	} else {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	var resp *http.Response
	var err error

	if urlType == "main" {
		resp, err = client.Get(targetUrl)
	} else {
		req, reqErr := http.NewRequest("GET", targetUrl, nil)
		if reqErr != nil {
			w.recordScrapedUrl(targetUrl, urlType, "", false, reqErr, 0, "")
			return nil, "", "", reqErr
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; WebSiteAssistantBot/1.0)")
		resp, err = client.Do(req)
	}

	if err != nil {
		w.recordScrapedUrl(targetUrl, urlType, "", false, err, 0, "")
		return nil, "", "", fmt.Errorf("failed to fetch URL %s: %v", targetUrl, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("HTTP %d", resp.StatusCode)
		w.recordScrapedUrl(targetUrl, urlType, "", false, err, 0, "")
		return nil, "", "", err
	}

	// Read the raw HTML content
	htmlBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		w.recordScrapedUrl(targetUrl, urlType, "", false, err, 0, "")
		return nil, "", "", fmt.Errorf("failed to read response body: %v", err)
	}

	htmlContent := string(htmlBytes)

	// Calculate content hash
	contentHash := w.calculateContentHash(htmlContent)

	// Parse HTML using strings.Reader since we already read the body
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		w.recordScrapedUrl(targetUrl, urlType, "", false, err, 0, "")
		return nil, "", "", fmt.Errorf("failed to parse HTML: %v", err)
	}

	title := strings.TrimSpace(doc.Find("title").First().Text())
	return doc, title, contentHash, nil
}

func (w *WebScraper) scrapeWebsiteWithDepth(targetUrl string, depth int) (*WebsiteContent, error) {
	// Try to load from disk first if refresh is not enabled
	if !w.refreshContent {
		if diskContent, err := w.loadContentFromDisk(targetUrl); err == nil {
			// Check if disk content is not too old
			if time.Since(diskContent.LastUpdated) < w.cacheDuration {
				w.recordScrapedUrl(targetUrl, "main", diskContent.Title, true, nil, 0, "disk_cached")
				w.cache[targetUrl] = *diskContent
				return diskContent, nil
			}
		}
	}

	// Check memory cache
	if cached, exists := w.cache[targetUrl]; exists {
		if time.Since(cached.LastUpdated) < w.memoryCacheDuration {
			w.recordScrapedUrl(targetUrl, "main", cached.Title, true, nil, 0, "memory_cached")
			return &cached, nil
		}
	}

	doc, title, contentHash, err := w.scrapePage(targetUrl, depth, "main", true)
	if err != nil {
		return nil, err
	}

	// Check if we already have content with the same hash
	if existingContent, err := w.findContentByHash(contentHash); err == nil {
		// Clone the existing content but update URL-specific fields
		content := *existingContent
		content.LastUpdated = time.Now()

		// Save to current URL's cache location and memory cache
		w.cache[targetUrl] = content
		if saveErr := w.saveContentToDisk(targetUrl, &content); saveErr != nil {
			fmt.Printf("Warning: Failed to save reused content to disk: %v\n", saveErr)
		}

		w.recordScrapedUrl(targetUrl, "main", content.Title, true, nil, 0, "content_reused")
		fmt.Printf("Reused existing content for: %s (hash: %s)\n", targetUrl, contentHash[:8])
		return &content, nil
	}

	content := WebsiteContent{
		LastUpdated:   time.Now(),
		PDFContent:    make(map[string]*PDFContent),
		FileContent:   make(map[string]*FileContent),
		LinkedContent: make(map[string]*LinkedPageContent),
		Metadata:      make(map[string]string),
		ContentHash:   contentHash,
	}

	content.Title = title

	// Extract meta information
	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		if name, exists := s.Attr("name"); exists {
			if cont, exists := s.Attr("content"); exists {
				switch name {
				case "description":
					content.Description = cont
				case "keywords":
					content.Metadata["keywords"] = cont
				case "author":
					content.Metadata["author"] = cont
				default:
					content.Metadata[name] = cont
				}
			}
		}
		if property, exists := s.Attr("property"); exists {
			if cont, exists := s.Attr("content"); exists {
				content.Metadata[property] = cont
			}
		}
	})

	var b strings.Builder
	b.Grow(10000) // Preallocate to avoid multiple allocations
	doc.Find("body").Each(func(i int, s *goquery.Selection) {
		walk(&b, s.Nodes[0], 0)
	})

	fullText := b.String()

	// Use Ollama to summarize the content if service is available
	if w.ollamaService != nil && w.ollamaService.IsEnabled() && fullText != "" {
		if summary, err := w.ollamaService.SummarizeContent(title, fullText); err == nil {
			content.Text = summary
			fmt.Printf("Content summarized for main page: %s\n", targetUrl)
		} else {
			fmt.Printf("Warning: Failed to summarize main page content: %v\n", err)
			// Fallback to truncated original content
			if len(fullText) > w.maxContentLength {
				content.Text = fullText[:w.maxContentLength] + "..."
			} else {
				content.Text = fullText
			}
		}
	} else {
		// No summarization available, use original logic
		if len(fullText) > w.maxContentLength {
			content.Text = fullText[:w.maxContentLength] + "..."
		} else {
			content.Text = fullText
		}
	}

	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists {
			linkType := "internal"
			if strings.HasPrefix(href, "http") {
				linkType = "external"
			}

			content.Links = append(content.Links, Link{
				URL:   href,
				Title: strings.TrimSpace(s.Text()),
				Type:  linkType,
			})
		}
	})

	w.processPDFs(&content, targetUrl)
	w.processFiles(&content, targetUrl)
	w.processLinkedContentWithDepth(&content, targetUrl, depth)

	// Record successful main page scraping
	w.recordScrapedUrl(targetUrl, "main", content.Title, true, nil, 0, "website")

	// Save content to disk
	if err := w.saveContentToDisk(targetUrl, &content); err != nil {
		fmt.Printf("Warning: Failed to save content to disk: %v\n", err)
	}

	w.cache[targetUrl] = content
	return &content, nil
}

func (w *WebScraper) processPDFs(content *WebsiteContent, baseURL string) {
	for _, link := range content.Links {
		if w.isPDFLink(link.URL) {
			fullURL := w.resolveURL(baseURL, link.URL)

			if cached, exists := w.pdfCache[fullURL]; exists {
				if time.Since(cached.LastUpdated) < w.cacheDuration {
					content.PDFContent[link.URL] = cached
					continue
				}
			}

			pdfContent, err := w.pdfExtractor.ExtractFromURL(fullURL)
			if err != nil {
				w.recordScrapedUrl(fullURL, "pdf", link.Title, false, err, 0, "pdf")
				continue
			}

			w.recordScrapedUrl(fullURL, "pdf", pdfContent.Title, true, nil, 0, "pdf")
			w.pdfCache[fullURL] = pdfContent
			content.PDFContent[link.URL] = pdfContent
		}
	}
}

func (w *WebScraper) processFiles(content *WebsiteContent, baseURL string) {
	for _, link := range content.Links {
		if w.isFileLink(link.URL) {
			fullURL := w.resolveURL(baseURL, link.URL)

			if cached, exists := w.fileCache[fullURL]; exists {
				if time.Since(cached.LastUpdated) < w.cacheDuration {
					content.FileContent[link.URL] = cached
					continue
				}
			}

			fileContent, err := w.fileParser.ParseFromURL(fullURL)
			if err != nil {
				w.recordScrapedUrl(fullURL, "file", link.Title, false, err, 0, "file")
				continue
			}

			w.recordScrapedUrl(fullURL, "file", fileContent.FileName, true, nil, 0, fileContent.FileType)
			w.fileCache[fullURL] = fileContent
			content.FileContent[link.URL] = fileContent
		}
	}
}

func (w *WebScraper) isPDFLink(url string) bool {
	return w.pdfExtractor.isValidPDFURL(url)
}

func (w *WebScraper) isFileLink(url string) bool {
	return w.fileParser.isValidFileURL(url)
}

func (w *WebScraper) resolveURL(baseURL, linkURL string) string {
	// If linkURL is already absolute, return as-is
	if strings.HasPrefix(linkURL, "http") {
		return linkURL
	}

	// Use Go's built-in URL resolution
	base, err := url.Parse(baseURL)
	if err != nil {
		// Fallback to simple string concatenation if parsing fails
		if strings.HasPrefix(linkURL, "/") {
			return strings.TrimSuffix(baseURL, "/") + linkURL
		}
		return strings.TrimSuffix(baseURL, "/") + "/" + linkURL
	}

	// Parse the relative URL
	relative, err := url.Parse(linkURL)
	if err != nil {
		// Fallback to simple string concatenation if parsing fails
		if strings.HasPrefix(linkURL, "/") {
			return strings.TrimSuffix(baseURL, "/") + linkURL
		}
		return strings.TrimSuffix(baseURL, "/") + "/" + linkURL
	}

	// Use Go's ResolveReference which handles relative URLs correctly
	resolved := base.ResolveReference(relative)
	result := resolved.String()

	// Debug logging to help troubleshoot URL resolution issues
	//fmt.Printf("DEBUG: URL Resolution - Base: %s, Link: %s, Result: %s\n", baseURL, linkURL, result)

	return result
}

//func (w *WebScraper) processLinkedContent(content *WebsiteContent, baseURL string) {
//	w.processLinkedContentWithDepth(content, baseURL, 0)
//}

func (w *WebScraper) processLinkedContentWithDepth(content *WebsiteContent, baseURL string, depth int) {
	// Check if we can continue scraping
	if depth >= w.maxScrapingDepth || !w.canScrapeMore() {
		return
	}

	// Mark current URL as visited
	w.markURLVisited(baseURL)

	// Process both professional links and internal navigation links
	for _, link := range content.Links {
		shouldProcess := false
		fullURL := link.URL

		// Resolve URLs to absolute URLs
		if link.Type == "internal" {
			fullURL = w.resolveURL(baseURL, link.URL)
		} else if strings.HasPrefix(link.URL, "/") {
			// Handle absolute paths that might be misclassified as external
			fullURL = w.resolveURL(baseURL, link.URL)
		}

		// Check if it's a professional link (external profiles)
		if w.isProfessionalLink(fullURL) {
			shouldProcess = true
		}

		// Check if it's an internal navigation link that's allowed by URL patterns
		if !shouldProcess && w.enableInternalLinks && w.isInternalNavigationLink(fullURL, link.Type) {
			shouldProcess = true
		}

		if shouldProcess {
			_, err := w.scrapeLinkedPageWithDepthAndContent(fullURL, depth+1, content)
			if err != nil {
				// Log error but continue processing other links
				fmt.Printf("Warning: Failed to scrape linked page %s: %v\n", fullURL, err)
			}

			//linkedContent, err := w.scrapeLinkedPageWithDepthAndContent(fullURL, depth+1, content)
			//if err == nil && linkedContent != nil {
			//	content.LinkedContent[fullURL] = linkedContent
			//}

			// Note: scrapeLinkedPageWithDepth handles its own recording and recursion
		}
	}
}

func (w *WebScraper) isProfessionalLink(url string) bool {
	professionalDomains := []string{
		"linkedin.com",
		"github.com",
		"gitlab.com",
		"stackoverflow.com",
		"medium.com",
		"dev.to",
		"twitter.com",
		"x.com",
	}

	lowerURL := strings.ToLower(url)
	for _, domain := range professionalDomains {
		if strings.Contains(lowerURL, domain) {
			return true
		}
	}
	return false
}

func (w *WebScraper) isInternalNavigationLink(fullUrl, linkType string) bool {
	// Only process internal links (not external)
	if linkType != "internal" {
		return false
	}

	// Check if the internal link would be allowed by URL patterns
	if !w.isUrlAllowed(fullUrl) {
		return false
	}

	// Skip certain common non-content links
	lowerUrl := strings.ToLower(fullUrl)
	skipPatterns := []string{
		"#", // anchor links
		"mailto:",
		"tel:",
		"javascript:",
		".css",
		".js",
		".ico",
		".png",
		".jpg",
		".jpeg",
		".gif",
		".svg",
		"/admin",
		"/login",
		"/logout",
		"/cart",
		"/checkout",
		"?search",
		"?sort",
		"?filter",
	}

	for _, pattern := range skipPatterns {
		if strings.Contains(lowerUrl, pattern) {
			return false
		}
	}

	return true
}

//func (w *WebScraper) scrapeLinkedPage(targetUrl string) (*LinkedPageContent, error) {
//	return w.scrapeLinkedPageWithDepth(targetUrl, 0)
//}

//func (w *WebScraper) scrapeLinkedPageWithDepth(targetUrl string, depth int) (*LinkedPageContent, error) {
//	return w.scrapeLinkedPageWithDepthAndContent(targetUrl, depth, nil)
//}

func (w *WebScraper) scrapeLinkedPageWithDepthAndContent(targetUrl string, depth int, mainContent *WebsiteContent) (*LinkedPageContent, error) {
	doc, title, contentHash, err := w.scrapePage(targetUrl, depth, "linked", false)
	if err != nil {
		return nil, err
	}

	// Check if we already have content with the same hash
	if existingContent, err := w.findContentByHash(contentHash); err == nil {
		// For linked content, we need to find existing LinkedPageContent
		// Search for existing linked page content with same hash
		for _, existingLinkedContent := range existingContent.LinkedContent {
			if existingLinkedContent.ContentHash == contentHash {
				// Clone and update the existing linked content
				linkedContent := *existingLinkedContent
				linkedContent.URL = targetUrl
				linkedContent.LastUpdated = time.Now()

				// Add to main content if provided
				if mainContent != nil {
					mainContent.LinkedContent[targetUrl] = &linkedContent
				}

				w.recordScrapedUrl(targetUrl, "linked", linkedContent.Title, true, nil, linkedContent.Relevance, "content_reused")
				fmt.Printf("Reused existing linked content for: %s (hash: %s)\n", targetUrl, contentHash[:8])
				return &linkedContent, nil
			}
		}
	}

	linkedContent := &LinkedPageContent{
		URL:             targetUrl,
		Title:           title,
		LastUpdated:     time.Now(),
		FirstLevelLinks: make([]FirstLevelLink, 0),
		ContentHash:     contentHash,
	}

	// Determine content type and relevance
	linkedContent.ContentType = w.determineContentType(targetUrl)
	//linkedContent.Relevance = w.calculateRelevance(targetUrl, linkedContent.Title)

	// Extract description
	doc.Find("meta[name='description'], meta[property='og:description']").Each(func(i int, s *goquery.Selection) {
		if desc, exists := s.Attr("content"); exists {
			if linkedContent.Description == "" {
				linkedContent.Description = desc
			}
		}
	})

	// Extract keywords
	doc.Find("meta[name='keywords']").Each(func(i int, s *goquery.Selection) {
		if keywords, exists := s.Attr("content"); exists {
			linkedContent.Keywords = strings.Split(keywords, ",")
			for i, keyword := range linkedContent.Keywords {
				linkedContent.Keywords[i] = strings.TrimSpace(keyword)
			}
		}
	})

	// Extract text content based on the platform
	if strings.Contains(targetUrl, "github.com") {
		// GitHub profile/repo specific selectors
		var textParts []string
		doc.Find(".user-profile-bio, .repository-description, .markdown-body, .readme").Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if text != "" && len(text) > w.minTextLength {
				textParts = append(textParts, text)
			}
		})
		linkedContent.Text = strings.Join(textParts, "\n\n")
	} else if strings.Contains(targetUrl, "linkedin.com") {
		// LinkedIn specific selectors (limited due to auth requirements)
		var textParts []string
		doc.Find(".pv-about-section, .summary, .experience").Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if text != "" && len(text) > w.minTextLength {
				textParts = append(textParts, text)
			}
		})
		linkedContent.Text = strings.Join(textParts, "\n\n")
	}
	//} else {
	//	// General content extraction
	//	//var textParts []string
	//	//doc.Find("p, h1, h2, h3, article, .content, .main, .bio, .about, .description").Each(func(i int, s *goquery.Selection) {
	//	//	text := strings.TrimSpace(s.Text())
	//	//	if text != "" && len(text) > w.minTextLength && len(text) < 1000 { // Reasonable text length
	//	//		textParts = append(textParts, text)
	//	//	}
	//	//})
	//	//linkedContent.Text = strings.Join(textParts, "\n\n")
	//	linkedContent.Text = doc.Text()
	//}

	var b strings.Builder
	b.Grow(10000) // Preallocate to avoid multiple allocations
	doc.Find("body").Each(func(i int, s *goquery.Selection) {
		walk(&b, s.Nodes[0], 0)
	})

	fullText := b.String()

	// Use Ollama to summarize the linked content if service is available
	if w.ollamaService != nil && w.ollamaService.IsEnabled() && fullText != "" {
		if summary, err := w.ollamaService.SummarizeContent(linkedContent.Title, fullText); err == nil {
			linkedContent.Text = summary
			fmt.Printf("Content summarized for linked page: %s\n", targetUrl)
		} else {
			fmt.Printf("Warning: Failed to summarize linked page content: %v\n", err)
			// Fallback to truncated original content
			if len(fullText) > w.maxContentLength {
				linkedContent.Text = fullText[:w.maxContentLength] + "..."
			} else {
				linkedContent.Text = fullText
			}
		}
	} else {
		// No summarization available, use original logic
		if len(fullText) > w.maxContentLength {
			linkedContent.Text = fullText[:w.maxContentLength] + "..."
		} else {
			linkedContent.Text = fullText
		}
	}

	// Process nested links recursively if we haven't reached max depth
	if depth+1 < w.maxScrapingDepth && w.canScrapeMore() {
		// Find and process external links from this page
		doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
			href, exists := s.Attr("href")
			if !exists {
				return
			}

			// Resolve relative URLs
			fullURL := href
			if strings.HasPrefix(href, "/") || strings.HasPrefix(href, "./") {
				fullURL = w.resolveURL(targetUrl, href)
			}

			// Skip if not HTTP/HTTPS
			if !strings.HasPrefix(fullURL, "http") {
				return
			}

			// Skip same domain links to avoid circular scraping
			if w.isSameDomain(targetUrl, fullURL) {
				return
			}

			// Skip if already visited
			if w.isURLVisited(fullURL) {
				return
			}

			// Skip if URL not allowed
			if !w.isUrlAllowed(fullURL) {
				return
			}

			// Recursively scrape this URL and add to the main content if available
			if nestedContent, err := w.scrapeLinkedPageWithDepthAndContent(fullURL, depth+1, mainContent); err == nil && nestedContent != nil {
				// If we have a main content structure, add this to it for access by the chatbot
				if mainContent != nil {
					mainContent.LinkedContent[fullURL] = nestedContent
				}
			} else if err != nil {
				// Log error but continue with other links
				log.Printf("Failed to scrape nested link %s at depth %d: %v", fullURL, depth+1, err)
			}
		})
	}

	// Record successful linked page scraping
	w.recordScrapedUrl(targetUrl, "linked", linkedContent.Title, true, nil, linkedContent.Relevance, linkedContent.ContentType)

	return linkedContent, nil
}

func walk(b *strings.Builder, n *html.Node, indent int) {
	if n.Type == html.ElementNode {
		tag := n.Data

		// Skip script/style
		if tag == "script" || tag == "style" || tag == "noscript" || tag == "frame" || tag == "iframe" || tag == "a" {
			return
		}

		// If the element has text, print it
		text := strings.TrimSpace(goquery.NewDocumentFromNode(n).Text())
		if text != "" {
			b.WriteString(fmt.Sprintf(" %s\n", text))
			//b.WriteString(fmt.Sprintf("%s%s\n", strings.Repeat(" ", indent), text))
			//b.WriteString(fmt.Sprintf("%s[%s] %s\n", strings.Repeat("  ", indent), tag, text))
		}

		// Recurse into children
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(b, c, indent+1)
		}
	}
}
func (w *WebScraper) determineContentType(url string) string {
	lowerURL := strings.ToLower(url)

	if strings.Contains(lowerURL, "github.com") || strings.Contains(lowerURL, "gitlab.com") {
		return "project"
	} else if strings.Contains(lowerURL, "linkedin.com") {
		return "professional"
	} else if strings.Contains(lowerURL, "medium.com") || strings.Contains(lowerURL, "dev.to") || strings.Contains(lowerURL, "blog") {
		return "blog"
	} else if strings.Contains(lowerURL, "stackoverflow.com") {
		return "technical"
	}

	return "general"
}

//func (w *WebScraper) calculateRelevance(url, title string) int {
//	relevance := 5 // Base relevance
//
//	lowerURL := strings.ToLower(url)
//	lowerTitle := strings.ToLower(title)
//
//	// Professional platforms get higher relevance
//	professionalKeywords := []string{"github", "linkedin", "gitlab", "portfolio", "resume", "cv"}
//	for _, keyword := range professionalKeywords {
//		if strings.Contains(lowerURL, keyword) || strings.Contains(lowerTitle, keyword) {
//			relevance += 2
//			break
//		}
//	}
//
//	// Technical content gets bonus
//	techKeywords := []string{"developer", "engineer", "programming", "code", "software", "tech"}
//	for _, keyword := range techKeywords {
//		if strings.Contains(lowerTitle, keyword) {
//			relevance += 1
//			break
//		}
//	}
//
//	// Blog/article content
//	blogKeywords := []string{"blog", "article", "tutorial", "guide"}
//	for _, keyword := range blogKeywords {
//		if strings.Contains(lowerURL, keyword) || strings.Contains(lowerTitle, keyword) {
//			relevance += 1
//			break
//		}
//	}
//
//	// Cap at 10
//	if relevance > 10 {
//		relevance = 10
//	}
//
//	return relevance
//}

func (w *WebScraper) isSameDomain(url1, url2 string) bool {
	// Simple domain comparison
	if strings.Contains(url1, "github.com") && strings.Contains(url2, "github.com") {
		return true
	}
	if strings.Contains(url1, "linkedin.com") && strings.Contains(url2, "linkedin.com") {
		return true
	}
	// Add more domain checks as needed
	return false
}

// parseHTMLFromURL fetches and parses HTML from a URL
func (w *WebScraper) parseHTMLFromURL(targetUrl string) (*goquery.Document, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", targetUrl, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; PersonalProfileBot/1.0)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return goquery.NewDocumentFromReader(resp.Body)
}
