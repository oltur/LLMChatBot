# CLAUDE.md

This file contains project-specific information for Claude Code to better understand and work with this codebase.

## Project Overview
LLM Chat Bot - An AI-powered chatbot application that provides information about John Smith using web scraping and local Ollama CodeLlama integration for intelligent content analysis.

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

## Features
- Enhanced web scraping  for comprehensive profile information
- Metadata extraction from website (keywords, author, descriptions, etc.)
- External professional profile scraping (GitHub, LinkedIn, GitLab, Medium, etc.)
- **First-level external page scraping** - automatically discovers and scrapes linked pages from professional profiles
- Content relevance scoring and type classification (professional, blog, project, technical)
- PDF content extraction and analysis
- Local Ollama CodeLlama integration for intelligent content analysis
- Multi-layered content aggregation: main site + external profiles + first-level links
- RESTful API endpoints for chat functionality
- Static web interface

## Important Notes
- Follow existing code conventions and patterns
- Run `go fmt` and `go vet` before committing
- Ensure Ollama is running locally with codellama:13b model to enable AI features
- Update this file as the project evolves