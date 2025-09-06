package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type WebScraper struct {
	client             *http.Client
	cache              map[string]WebsiteContent
	pdfExtractor       *PDFExtractor
	pdfCache           map[string]*PDFContent
	fileParser         *FileParser
	fileCache          map[string]*FileContent
	allowedUrlPatterns []string
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

	return &WebScraper{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		cache:              make(map[string]WebsiteContent),
		pdfExtractor:       NewPDFExtractor(),
		pdfCache:           make(map[string]*PDFContent),
		fileParser:         NewFileParser(),
		fileCache:          make(map[string]*FileContent),
		allowedUrlPatterns: allowedUrlPatterns,
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

func (w *WebScraper) ScrapeWebsite(targetUrl string) (*WebsiteContent, error) {
	// Check if the URL is allowed to be scraped
	if !w.isUrlAllowed(targetUrl) {
		return nil, fmt.Errorf("URL not allowed for scraping: %s", targetUrl)
	}

	if cached, exists := w.cache[targetUrl]; exists {
		if time.Since(cached.LastUpdated) < 1*time.Hour {
			return &cached, nil
		}
	}

	resp, err := w.client.Get(targetUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL %s: %v", targetUrl, err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
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
	w.processLinkedContent(&content)

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
				continue
			}

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
				continue
			}

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
	if strings.HasPrefix(linkURL, "http") {
		return linkURL
	}

	if strings.HasPrefix(linkURL, "/") {
		return strings.TrimSuffix(baseURL, "/") + linkURL
	}

	return strings.TrimSuffix(baseURL, "/") + "/" + linkURL
}

func (w *WebScraper) processLinkedContent(content *WebsiteContent) {
	// Process external professional links for additional context
	for _, link := range content.Links {
		if w.isProfessionalLink(link.URL) {
			linkedContent, err := w.scrapeLinkedPage(link.URL)
			if err == nil && linkedContent != nil {
				content.LinkedContent[link.URL] = linkedContent
			}
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

func (w *WebScraper) scrapeLinkedPage(targetUrl string) (*LinkedPageContent, error) {
	// Check if the URL is allowed to be scraped
	if !w.isUrlAllowed(targetUrl) {
		return nil, fmt.Errorf("URL not allowed for scraping: %s", targetUrl)
	}

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	req, err := http.NewRequest("GET", targetUrl, nil)
	if err != nil {
		return nil, err
	}

	// Add user agent to avoid being blocked
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; PersonalProfileBot/1.0)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
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
		return nil
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; PersonalProfileBot/1.0)")

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
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
		return firstLevelLink
	}

	return nil
}
