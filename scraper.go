package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
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

func NewWebScraper() *WebScraper {
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
	}
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
	// Check if the URL is allowed to be scraped
	if !w.isUrlAllowed(targetUrl) {
		err := fmt.Errorf("URL not allowed for scraping: %s", targetUrl)
		w.recordScrapedUrl(targetUrl, "main", "", false, err, 0, "")
		return nil, err
	}

	if cached, exists := w.cache[targetUrl]; exists {
		if time.Since(cached.LastUpdated) < 1*time.Hour {
			w.recordScrapedUrl(targetUrl, "main", cached.Title, true, nil, 0, "cached")
			return &cached, nil
		}
	}

	resp, err := w.client.Get(targetUrl)
	if err != nil {
		w.recordScrapedUrl(targetUrl, "main", "", false, err, 0, "")
		return nil, fmt.Errorf("failed to fetch URL %s: %v", targetUrl, err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		w.recordScrapedUrl(targetUrl, "main", "", false, err, 0, "")
		return nil, fmt.Errorf("failed to parse HTML: %v", err)
	}

	content := WebsiteContent{
		LastUpdated:   time.Now(),
		PDFContent:    make(map[string]*PDFContent),
		FileContent:   make(map[string]*FileContent),
		LinkedContent: make(map[string]*LinkedPageContent),
		Metadata:      make(map[string]string),
	}

	content.Title = strings.TrimSpace(doc.Find("title").First().Text())

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

	// Extract comprehensive text content
	var textParts []string
	doc.Find("p, h1, h2, h3, h4, h5, h6, article, section, div.content, div.main").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text != "" && len(text) > 10 { // Filter out very short text
			textParts = append(textParts, text)
		}
	})
	content.Text = strings.Join(textParts, "\n\n")

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
	w.processLinkedContent(&content, targetUrl)

	// Record successful main page scraping
	w.recordScrapedUrl(targetUrl, "main", content.Title, true, nil, 0, "website")

	w.cache[targetUrl] = content
	return &content, nil
}

func (w *WebScraper) processPDFs(content *WebsiteContent, baseURL string) {
	for _, link := range content.Links {
		if w.isPDFLink(link.URL) {
			fullURL := w.resolveURL(baseURL, link.URL)

			if cached, exists := w.pdfCache[fullURL]; exists {
				if time.Since(cached.LastUpdated) < 24*time.Hour {
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
				if time.Since(cached.LastUpdated) < 24*time.Hour {
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
	fmt.Printf("DEBUG: URL Resolution - Base: %s, Link: %s, Result: %s\n", baseURL, linkURL, result)

	return result
}

func (w *WebScraper) processLinkedContent(content *WebsiteContent, baseURL string) {
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
			linkedContent, err := w.scrapeLinkedPage(fullURL)
			if err == nil && linkedContent != nil {
				content.LinkedContent[fullURL] = linkedContent
			}
			// Note: scrapeLinkedPage handles its own recording
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

func (w *WebScraper) scrapeLinkedPage(targetUrl string) (*LinkedPageContent, error) {
	// Check if the URL is allowed to be scraped
	if !w.isUrlAllowed(targetUrl) {
		err := fmt.Errorf("URL not allowed for scraping: %s", targetUrl)
		w.recordScrapedUrl(targetUrl, "linked", "", false, err, 0, "")
		return nil, err
	}

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	req, err := http.NewRequest("GET", targetUrl, nil)
	if err != nil {
		w.recordScrapedUrl(targetUrl, "linked", "", false, err, 0, "")
		return nil, err
	}

	// Add user agent to avoid being blocked
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; PersonalProfileBot/1.0)")

	resp, err := client.Do(req)
	if err != nil {
		w.recordScrapedUrl(targetUrl, "linked", "", false, err, 0, "")
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("HTTP %d", resp.StatusCode)
		w.recordScrapedUrl(targetUrl, "linked", "", false, err, 0, "")
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		w.recordScrapedUrl(targetUrl, "linked", "", false, err, 0, "")
		return nil, err
	}

	linkedContent := &LinkedPageContent{
		URL:             targetUrl,
		LastUpdated:     time.Now(),
		FirstLevelLinks: make([]FirstLevelLink, 0),
	}

	// Extract title
	linkedContent.Title = strings.TrimSpace(doc.Find("title").First().Text())

	// Determine content type and relevance
	linkedContent.ContentType = w.determineContentType(targetUrl)
	linkedContent.Relevance = w.calculateRelevance(targetUrl, linkedContent.Title)

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
			if text != "" && len(text) > 10 {
				textParts = append(textParts, text)
			}
		})
		linkedContent.Text = strings.Join(textParts, "\n\n")
	} else if strings.Contains(targetUrl, "linkedin.com") {
		// LinkedIn specific selectors (limited due to auth requirements)
		var textParts []string
		doc.Find(".pv-about-section, .summary, .experience").Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if text != "" && len(text) > 10 {
				textParts = append(textParts, text)
			}
		})
		linkedContent.Text = strings.Join(textParts, "\n\n")
	} else {
		// General content extraction
		var textParts []string
		doc.Find("p, h1, h2, h3, article, .content, .main, .bio, .about, .description").Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if text != "" && len(text) > 10 && len(text) < 1000 { // Reasonable text length
				textParts = append(textParts, text)
			}
		})
		linkedContent.Text = strings.Join(textParts, "\n\n")
	}

	// Limit content size to avoid overwhelming the AI
	if len(linkedContent.Text) > 2000 {
		linkedContent.Text = linkedContent.Text[:2000] + "..."
	}

	// Scrape first-level external links
	w.scrapeFirstLevelLinks(doc, linkedContent)

	// Record successful linked page scraping
	w.recordScrapedUrl(targetUrl, "linked", linkedContent.Title, true, nil, linkedContent.Relevance, linkedContent.ContentType)

	return linkedContent, nil
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

func (w *WebScraper) calculateRelevance(url, title string) int {
	relevance := 5 // Base relevance

	lowerURL := strings.ToLower(url)
	lowerTitle := strings.ToLower(title)

	// Professional platforms get higher relevance
	professionalKeywords := []string{"github", "linkedin", "gitlab", "portfolio", "resume", "cv"}
	for _, keyword := range professionalKeywords {
		if strings.Contains(lowerURL, keyword) || strings.Contains(lowerTitle, keyword) {
			relevance += 2
			break
		}
	}

	// Technical content gets bonus
	techKeywords := []string{"developer", "engineer", "programming", "code", "software", "tech"}
	for _, keyword := range techKeywords {
		if strings.Contains(lowerTitle, keyword) {
			relevance += 1
			break
		}
	}

	// Blog/article content
	blogKeywords := []string{"blog", "article", "tutorial", "guide"}
	for _, keyword := range blogKeywords {
		if strings.Contains(lowerURL, keyword) || strings.Contains(lowerTitle, keyword) {
			relevance += 1
			break
		}
	}

	// Cap at 10
	if relevance > 10 {
		relevance = 10
	}

	return relevance
}

func (w *WebScraper) scrapeFirstLevelLinks(doc *goquery.Document, linkedContent *LinkedPageContent) {
	// Extract external links from the current page
	var firstLevelLinks []FirstLevelLink
	maxLinks := 5 // Limit to prevent overwhelming data

	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		if len(firstLevelLinks) >= maxLinks {
			return
		}

		href, exists := s.Attr("href")
		if !exists {
			return
		}

		// Make URL absolute if relative
		if strings.HasPrefix(href, "/") || strings.HasPrefix(href, "./") {
			return // Skip relative links for now
		}

		// Skip if not HTTP/HTTPS
		if !strings.HasPrefix(href, "http") {
			return
		}

		// Skip same domain as the current page
		if w.isSameDomain(linkedContent.URL, href) {
			return
		}

		// Skip if not relevant
		linkTitle := strings.TrimSpace(s.Text())
		relevance := w.calculateRelevance(href, linkTitle)

		if relevance < 6 { // Only include moderately relevant links
			return
		}

		// Try to scrape the first-level page
		firstLevelContent := w.scrapeFirstLevelPage(href, linkTitle)
		if firstLevelContent != nil {
			firstLevelLinks = append(firstLevelLinks, *firstLevelContent)
		}
		// Note: scrapeFirstLevelPage handles its own recording
	})

	linkedContent.FirstLevelLinks = firstLevelLinks
}

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

func (w *WebScraper) scrapeFirstLevelPage(targetUrl, title string) *FirstLevelLink {
	// Check if the URL is allowed to be scraped
	if !w.isUrlAllowed(targetUrl) {
		return nil // Silently skip disallowed URLs for first-level links
	}

	client := &http.Client{
		Timeout: 10 * time.Second, // Shorter timeout for first-level links
	}

	req, err := http.NewRequest("GET", targetUrl, nil)
	if err != nil {
		w.recordScrapedUrl(targetUrl, "first_level", title, false, err, 0, "first_level")
		return nil
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; PersonalProfileBot/1.0)")

	resp, err := client.Do(req)
	if err != nil {
		w.recordScrapedUrl(targetUrl, "first_level", title, false, err, 0, "first_level")
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("HTTP %d", resp.StatusCode)
		w.recordScrapedUrl(targetUrl, "first_level", title, false, err, 0, "first_level")
		return nil
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		w.recordScrapedUrl(targetUrl, "first_level", title, false, err, 0, "first_level")
		return nil
	}

	firstLevelLink := &FirstLevelLink{
		URL:         targetUrl,
		Title:       title,
		LastUpdated: time.Now(),
	}

	// If title is empty, try to extract from the page
	if firstLevelLink.Title == "" {
		firstLevelLink.Title = strings.TrimSpace(doc.Find("title").First().Text())
	}

	// Extract description
	doc.Find("meta[name='description'], meta[property='og:description']").Each(func(i int, s *goquery.Selection) {
		if desc, exists := s.Attr("content"); exists {
			if firstLevelLink.Description == "" {
				firstLevelLink.Description = desc
			}
		}
	})

	// Extract limited text content
	var textParts []string
	doc.Find("h1, h2, h3, p").Each(func(i int, s *goquery.Selection) {
		if len(textParts) >= 5 { // Limit to 5 text elements
			return
		}
		text := strings.TrimSpace(s.Text())
		if text != "" && len(text) > 20 && len(text) < 500 {
			textParts = append(textParts, text)
		}
	})

	firstLevelLink.Text = strings.Join(textParts, "\n\n")

	// Limit total size
	if len(firstLevelLink.Text) > 1000 {
		firstLevelLink.Text = firstLevelLink.Text[:1000] + "..."
	}

	firstLevelLink.Relevance = w.calculateRelevance(targetUrl, firstLevelLink.Title)

	// Only return if there's meaningful content
	if len(firstLevelLink.Text) > 50 || len(firstLevelLink.Description) > 20 {
		// Record successful first-level page scraping
		w.recordScrapedUrl(targetUrl, "first_level", firstLevelLink.Title, true, nil, firstLevelLink.Relevance, "first_level")
		return firstLevelLink
	}

	// Record failed first-level page scraping (insufficient content)
	w.recordScrapedUrl(targetUrl, "first_level", firstLevelLink.Title, false, fmt.Errorf("insufficient content"), 0, "first_level")
	return nil
}
