# AI-Powered Chatbot

An advanced Go-based chatbot application that provides comprehensive information about websites through multi-layered web scraping, PDF analysis, and local AI integration using Ollama CodeLlama.

## 🚀 Features

### Core Capabilities
- **Enhanced Web Scraping** of specified web site with comprehensive metadata extraction
- **Persistent Content Storage** with directory-based caching per website URL
- **Smart Caching System** with configurable refresh behavior for optimal performance
- **PDF Analysis & Extraction** with intelligent content parsing (CV/Resume)
- **Local AI Integration** using Ollama CodeLlama for intelligent responses
- **Multi-layered Content Aggregation** from website + external profiles + first-level links
- **RESTful API** for chat interactions
- **Responsive Web Interface** for easy interaction

### Advanced Scraping Features
- **Recursive Link Following**: Configurable depth scraping (1-10 levels) with loop detection
- **External Profile Scraping**: Automatically discovers and scrapes professional profiles (GitHub, LinkedIn, GitLab, Medium, Dev.to, StackOverflow, Twitter/X)
- **Multi-Level Link Discovery**: Recursively follows links up to configured depth with intelligent filtering
- **Loop Detection**: URL normalization and visited tracking prevents infinite scraping loops
- **Content Relevance Scoring**: 1-10 relevance system to prioritize high-quality information
- **Content Type Classification**: Categorizes content as professional, blog, project, technical, or general
- **Persistent Storage**: Content saved to `scraped_content/` directory, organized by website URL
- **Smart Caching System**: 24-hour disk caching with optional forced refresh via `REFRESH_CONTENT`

### AI-Powered Intelligence
- **Comprehensive Context Provision**: All scraped content provided to Ollama for analysis
- **Cross-Reference Capability**: AI can correlate information across multiple sources
- **Source Attribution**: Responses include relevance scores and source citations
- **Fallback System**: Rule-based responses if AI is unavailable

## 📋 Prerequisites

- **Go 1.21+**
- **Ollama** installed and running locally
- **CodeLlama:13b model** pulled in Ollama
- **WEBSITE_URL environment variable** must be set before running

### Installing Ollama and CodeLlama

```bash
# Install Ollama (macOS)
curl -fsSL https://ollama.com/install.sh | sh

# Pull CodeLlama 13B model
ollama pull codellama:13b

# Verify installation
ollama list
```

## 🛠 Installation

1. **Clone the repository**
```bash
git clone <repository-url>
cd LLMChatBot
```

2. **Install Go dependencies**
```bash
go mod tidy
```

3. **Build the application**
```bash
go build
```

## 🚦 Usage

### Running the Application

⚠️ **Important**: The `WEBSITE_URL` environment variable is required. The application will exit with an error if not provided.

```bash
# Basic configuration (WEBSITE_URL is required)
WEBSITE_URL=https://example.com ./chatbot

# Custom website URL
WEBSITE_URL=https://example.com ./chatbot

# Force refresh content (bypass disk cache)
WEBSITE_URL=https://example.com REFRESH_CONTENT=true ./chatbot

# Custom Ollama configuration
WEBSITE_URL=https://example.com OLLAMA_URL=http://localhost:11434 OLLAMA_MODEL=codellama:13b ./chatbot

# Full custom configuration with refresh
WEBSITE_URL=https://example.com OLLAMA_URL=http://localhost:11434 OLLAMA_MODEL=codellama:13b PORT=8084 REFRESH_CONTENT=true ./chatbot
```

### Web Interface

Navigate to: `http://localhost:8080`

The web interface provides an intuitive chat experience with AI-powered responses.

### API Endpoints

#### Chat Endpoint
```bash
POST /chat
Content-Type: application/json

{
  "message": "What are the technical skills from GitHub and CV?"
}
```

**Response:**
```json
{
  "response": "Based on the CV and GitHub profiles, the technical skills include: [AI-generated comprehensive analysis of skills from multiple sources including CV, GitHub repositories, and linked projects]",
  "timestamp": "2025-09-05 20:42:37"
}
```

#### Health Check
```bash
GET /health
```

## 💬 Query Capabilities

### Basic Information Queries
- **Professional Background**: "What can you tell me about this person?"
- **Contact Information**: "How can I contact this person?"
- **Profile Links**: "What are the social profiles?"

### Advanced AI-Powered Queries
- **Technical Skills**: "What programming languages and technologies are mentioned?" *(Analyzes CV + GitHub + project links)*
- **Project Analysis**: "Tell me about recent projects" *(Cross-references GitHub + linked repositories)*
- **Career Progression**: "What's the professional experience?" *(Combines CV + LinkedIn + external content)*
- **Educational Background**: "Tell me about the education background" *(Extracts from CV + professional profiles)*
- **Blog & Articles**: "What topics are written about?" *(Analyzes blog posts + Medium articles + linked content)*

### Content Sources Used by AI
- **Main Website Content** with metadata
- **CV/Resume PDFs** with extracted skills, experience, education
- **GitHub Profiles** with repository information
- **LinkedIn Professional Content** (when accessible)
- **Blog Posts & Articles** from Medium, Dev.to, personal blogs
- **First-Level Linked Content** from all external profiles
- **Cross-Referenced Information** from multiple sources with relevance scoring

## 🏗 Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Web Scraper   │────│  Content Cache   │────│   AI Service    │
│  - Main site    │    │  - 1h web cache  │    │  - Ollama       │
│  - External     │    │  - 24h PDF cache │    │  - CodeLlama    │
│  - First-level  │    │  - Relevance     │    │  - Local AI     │
└─────────────────┘    └──────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌─────────────────┐
                    │   HTTP Server   │
                    │  - REST API     │
                    │  - Web UI       │
                    │  - Health Check │
                    └─────────────────┘
```

### File Structure
- **main.go**: Application entry point and dependency injection
- **scraper.go**: Multi-layered web scraping with first-level link discovery
- **ollama_service.go**: Local AI integration with Ollama CodeLlama
- **pdf_extractor.go**: PDF content extraction and analysis
- **chatbot.go**: Intelligence routing and response generation
- **server.go**: HTTP server and API endpoints
- **static/index.html**: Interactive web interface

## 🔧 Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `WEBSITE_URL` | Target website URL to scrape | **Required** |
| `PORT` | Server port | `8080` |
| `OLLAMA_URL` | Ollama API endpoint | `http://localhost:11434` |
| `OLLAMA_MODEL` | AI model to use | `codellama:13b` |
| `REFRESH_CONTENT` | Force refresh content on every request | `false` |
| `MIN_TEXT_LENGTH` | Minimum text length for content scraping | `10` |
| `MAX_TEXT_LENGTH` | Maximum text length for content scraping | `10000` |
| `MAX_SCRAPING_DEPTH` | Maximum recursive scraping depth (1-10) | `2` |
| `MAX_PAGES_PER_SESSION` | Maximum pages to scrape per session | `100` |
| `ALLOWED_SCRAPING_URL_PATTERNS` | Comma-separated URL patterns for scraping | All URLs allowed |
| `ENABLE_INTERNAL_LINK_SCRAPING` | Enable internal navigation link scraping | `false` |

### Content Storage & Caching

- **Storage Location**: `scraped_content/` directory with separate folders per website
- **Directory Structure**: `{domain}_{path_hash}/content.json`
- **Cache Duration**: 24 hours for disk storage, 1 hour for memory cache
- **Cache Control**: Set `REFRESH_CONTENT=true` to force fresh scraping
- **Content Format**: JSON with metadata, timestamps, and structured data
- **Performance**: Subsequent visits load from disk cache for faster response

### Content Scraping Limits

- **Recursive depth**: Configurable via `MAX_SCRAPING_DEPTH` (default: 2, max: 10)
- **Session limits**: `MAX_PAGES_PER_SESSION` prevents runaway scraping (default: 100)
- **Text length filtering**: `MIN_TEXT_LENGTH` (default: 10) and `MAX_TEXT_LENGTH` (default: 10000)
- **Loop prevention**: URL normalization and visited tracking
- **First-level links per profile**: 5 (prevents overwhelming data)
- **Minimum relevance threshold**: 6/10 (ensures quality)
- **Content size limits**: 2000 chars per profile, 1000 chars per first-level link
- **Professional platforms monitored**: GitHub, LinkedIn, GitLab, Medium, Dev.to, StackOverflow, Twitter/X

## 📊 Data Flow

1. **Website Scraping**: Extracts content + metadata
2. **External Profile Discovery**: Identifies professional links
3. **Profile Content Extraction**: Scrapes each external profile with enhanced metadata
4. **First-Level Link Analysis**: Discovers and scores outbound links from profiles
5. **Selective Deep Scraping**: Scrapes high-relevance (6+) first-level pages
6. **Content Aggregation**: Compiles all content with source attribution
7. **AI Processing**: Provides comprehensive context to Ollama CodeLlama
8. **Intelligent Response**: Generates responses using all available information

## 🚨 Status Indicators

### ✅ Implemented Features
- Multi-layered web scraping with first-level external page discovery
- Content relevance scoring and type classification
- PDF analysis and intelligent content extraction
- Local Ollama CodeLlama integration
- Comprehensive content aggregation across all sources
- RESTful API with enhanced response capabilities
- Responsive web interface with AI-powered chat
- Intelligent caching system
- Cross-source information correlation
- Fallback to rule-based responses when AI unavailable

### 🔄 AI Integration Status
- **AI Enabled**: When Ollama is running with CodeLlama:13b
- **Enhanced Responses**: Comprehensive analysis using all scraped content
- **Source Attribution**: Responses include relevance and source information
- **Fallback Available**: Rule-based responses when AI is unavailable

## 🎯 Example Use Cases

**Project Discovery**: "What projects has he worked on recently?"
*→ AI analyzes GitHub profile + linked repositories + project descriptions*

**Skill Assessment**: "Is he experienced with Go programming?"
*→ Cross-references CV + GitHub repos + blog posts + technical articles*

**Career Overview**: "Tell me about the professional background"
*→ Combines CV content + LinkedIn + blog posts + external references*

**Contact & Networking**: "How can I connect professionally?"
*→ Provides contact info + professional profiles with context*

## 🔧 Development Commands

```bash
# Build
go build

# Run
go run main.go

# Test
go test ./...

# Format code
go fmt ./...

# Vet code
go vet ./...
```

## 📦 Dependencies

- `github.com/gorilla/mux`: HTTP routing and middleware
- `github.com/PuerkitoBio/goquery`: HTML parsing and CSS selectors
- `github.com/ledongthuc/pdf`: PDF document parsing and text extraction

## 🤖 AI Model Information

- **Model**: CodeLlama:13b
- **Provider**: Ollama (local)
- **Capabilities**: Code analysis, technical content understanding, multi-source information synthesis
- **Context**: Receives comprehensive content from website + external profiles + first-level links
- **Response Quality**: Enhanced by relevance scoring and content type classification

---

**Ready for production use with comprehensive AI-powered content analysis and multi-layered web intelligence.**