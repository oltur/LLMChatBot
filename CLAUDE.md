# CLAUDE.md

This file contains project-specific information for Claude Code to better understand and work with this codebase.

## Project Overview
LLM Chat Bot - An AI-powered chatbot application that provides information about websites using web scraping and local Ollama CodeLlama integration for intelligent content analysis.

## Development Commands
- Build: `go build`
- Run: `go run main.go`
- Test: `go test ./...`
- Format: `go fmt ./...`
- Vet: `go vet ./...`

## Project Structure
```
LLMChatBot/
├── main.go           # Main entry point
├── chatbot.go        # Chatbot logic
├── server.go         # HTTP server
├── scraper.go        # Web scraping functionality
├── pdf_extractor.go  # PDF processing
├── ollama_service.go # Ollama API integration
├── static/           # Static web files
├── go.mod           # Go module definition
└── go.sum           # Go dependencies
```

## Environment Variables
- `WEBSITE_URL`: Target website URL to scrape (required)
- `OLLAMA_URL`: URL for Ollama API (defaults to http://localhost:11434)
- `OLLAMA_MODEL`: Model to use (defaults to codellama:13b)
- `PORT`: Server port (defaults to 8080)
- `ALLOWED_SCRAPING_URL_PATTERNS`: Comma-separated list of URL patterns allowed for scraping (optional, if not set allows all URLs)
- `ENABLE_INTERNAL_LINK_SCRAPING`: Set to "true" to enable scraping of internal navigation links, not just external professional links (default: false)
- `REFRESH_CONTENT`: Set to "true" to force refresh of scraped content on every request, "false" to use cached content from disk (default: false for speed)
- `MIN_TEXT_LENGTH`: Minimum length of text fragments to include during scraping (default: 10 characters)
- `MAX_CONTENT_LENGTH`: Maximum length of text fragments to include during scraping (default: 10000 characters)
- `MAX_SCRAPING_DEPTH`: How many levels deep to recursively follow links (default: 2, max: 10)
- `MAX_PAGES_PER_SESSION`: Safety limit for maximum pages scraped in one session (default: 100)
- `CACHE_DURATION_HOURS`: How long to keep cached content before refreshing (default: 24 hours)

## Features
- Enhanced web scraping for comprehensive profile information
- **Persistent content storage** - scraped content is saved to separate directories per website URL for faster subsequent access
- **Smart caching system** - uses disk-based persistence with configurable refresh behavior
- Metadata extraction from website (keywords, author, descriptions, etc.)
- External professional profile scraping (GitHub, LinkedIn, GitLab, Medium, etc.)
- **First-level external page scraping** - automatically discovers and scrapes linked pages from professional profiles
- Content relevance scoring and type classification (professional, blog, project, technical)
- PDF content extraction and analysis
- Local Ollama CodeLlama integration for intelligent content analysis
- Multi-layered content aggregation: main site + external profiles + first-level links
- RESTful API endpoints for chat functionality
- Static web interface

## Content Storage
The application automatically saves scraped content to the `scraped_content/` directory:
- Each website gets its own subdirectory based on the URL (domain + path hash)
- Content is stored in JSON format for fast loading
- By default, cached content is used for 24 hours to improve performance
- Set `REFRESH_CONTENT=true` to force fresh scraping on every request
- Content includes: main page, linked profiles, PDFs, and metadata

## Scraping Configuration
- **Recursive Depth**: `MAX_SCRAPING_DEPTH` controls how deep to follow links
  - Level 1: Only main page
  - Level 2: Main page + direct links (default, fast)
  - Level 3-5: Multi-level recursive scraping (comprehensive but slower)
  - Level 6-10: Deep scraping (use with caution, can be very slow)
  - **Loop Protection**: URL normalization and visited tracking prevents infinite loops
- **Session Limits**: `MAX_PAGES_PER_SESSION` prevents runaway scraping
- **Text Filtering**: Control text fragment size with `MIN_TEXT_LENGTH`
  - `MIN_TEXT_LENGTH` (default: 10): Higher values reduce noise, lower values capture more detail
  - Both settings affect all text extraction: main pages, external profiles, and linked content
  - Recommended ranges: MIN (5-30), MAX (1000-50000) depending on content type

## Important Notes
- Follow existing code conventions and patterns
- Run `go fmt` and `go vet` before committing
- Ensure Ollama is running locally with codellama:13b model to enable AI features
- The `scraped_content/` directory will be created automatically and can be safely deleted to clear all cached content
- Update this file as the project evolves