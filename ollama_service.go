package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	regexp "regexp"
	"strconv"
	"strings"
	"time"
)

type OllamaService struct {
	baseURL               string
	model                 string
	maxTotalContentLength int // Max length of content to send to Ollama
	client                *http.Client
}

type OllamaOptions struct {
	Seed        int     `json:"seed"`
	Temperature float64 `json:"temperature"`
	NumCtx      int     `json:"num_ctx"`
	NumPredict  int     `json:"num_predict"`
}

type OllamaRequest struct {
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt"`
	Stream  bool           `json:"stream"`
	Options *OllamaOptions `json:"options,omitempty"`
}

type OllamaResponse struct {
	Model     string `json:"model"`
	Response  string `json:"response"`
	Done      bool   `json:"done"`
	CreatedAt string `json:"created_at"`
}

func NewOllamaService() *OllamaService {
	baseURL := os.Getenv("OLLAMA_URL")
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	model := os.Getenv("OLLAMA_MODEL")
	if model == "" {
		model = "codellama:13b"
	}

	// Parse maximum total text length (default: 20000)
	maxTotalContentLength := 20000
	if maxContentLengthStr := os.Getenv("MAX_TOTAL_CONTENT_LENGTH"); maxContentLengthStr != "" {
		if parsed, err := strconv.Atoi(maxContentLengthStr); err == nil {
			maxTotalContentLength = parsed
		}
	}

	return &OllamaService{
		baseURL:               baseURL,
		model:                 model,
		maxTotalContentLength: maxTotalContentLength,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (s *OllamaService) IsEnabled() bool {
	// Test if Ollama is running by making a quick request to the API
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", s.baseURL+"/api/tags", nil)
	if err != nil {
		return false
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

func (s *OllamaService) generateResponse(prompt string) (string, error) {
	reqBody := OllamaRequest{
		Model:  s.model,
		Prompt: prompt,
		Stream: false,
		Options: &OllamaOptions{
			Seed:        42,
			Temperature: 0,
			NumCtx:      4096,
			NumPredict:  512,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", s.baseURL+"/api/generate", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama API error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama API returned status code: %d", resp.StatusCode)
	}

	var ollamaResp OllamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %v", err)
	}

	if ollamaResp.Response == "" {
		return "", fmt.Errorf("no response from Ollama API")
	}

	return ollamaResp.Response, nil
}

func (s *OllamaService) AnalyzePDFContent(pdfContent *PDFContent, question string) (string, error) {
	if !s.IsEnabled() {
		return "", fmt.Errorf("Ollama service is not available - ensure Ollama is running with %s model", s.model)
	}

	if pdfContent == nil {
		return "", fmt.Errorf("no PDF content provided")
	}

	content := pdfContent.Text

	prompt := fmt.Sprintf(`You are an AI assistant analyzing a CV/Resume. 

CV Content:
%s

User Question: %s

Please analyze the CV content and provide a comprehensive answer. 
Focus on extracting relevant information about skills, experience, education, and achievements.
`, content, question)

	return s.generateResponse(prompt)
}

func (s *OllamaService) AnalyzeFileContent(fileContent *FileContent, question string) (string, error) {
	if !s.IsEnabled() {
		return "", fmt.Errorf("Ollama service is not available - ensure Ollama is running with %s model", s.model)
	}

	if fileContent == nil {
		return "", fmt.Errorf("no file content provided")
	}

	var contentBuilder strings.Builder
	contentBuilder.WriteString(fmt.Sprintf("FILE TYPE: %s\n", strings.ToUpper(fileContent.FileType)))
	contentBuilder.WriteString(fmt.Sprintf("FILE NAME: %s\n", fileContent.FileName))

	if len(fileContent.SheetNames) > 0 {
		contentBuilder.WriteString(fmt.Sprintf("SHEETS: %s\n", strings.Join(fileContent.SheetNames, ", ")))
	}
	if fileContent.RowCount > 0 {
		contentBuilder.WriteString(fmt.Sprintf("ROWS: %d\n", fileContent.RowCount))
	}
	if fileContent.ColumnCount > 0 {
		contentBuilder.WriteString(fmt.Sprintf("COLUMNS: %d\n", fileContent.ColumnCount))
	}

	if len(fileContent.Metadata) > 0 {
		contentBuilder.WriteString("\nMETADATA:\n")
		for key, value := range fileContent.Metadata {
			contentBuilder.WriteString(fmt.Sprintf("- %s: %s\n", key, value))
		}
	}

	contentBuilder.WriteString("\nCONTENT:\n")
	contentBuilder.WriteString(fileContent.Text)

	prompt := fmt.Sprintf(`You are an AI assistant analyzing a %s file. 

FILE INFORMATION:
%s

User Question: %s

INSTRUCTIONS:
1. Analyze the file content based on its type (%s)
2. For XLSX files: Focus on data structure, patterns, and insights from spreadsheet data
3. For DOCX files: Extract key information, document structure, and textual content
4. For CSV files: Identify data patterns, column relationships, and statistical insights
5. Provide relevant answers based on the file content and user's question
6. If the file contains professional data (resume, portfolio, etc.), highlight relevant skills and experience
7. For data files, provide summaries and key findings

Please provide a comprehensive analysis based on the file content above.`, strings.ToUpper(fileContent.FileType), contentBuilder.String(), question, strings.ToUpper(fileContent.FileType))

	return s.generateResponse(prompt)
}

func (s *OllamaService) GenerateIntelligentResponse(websiteContent *WebsiteContent, userMessage string) (string, error) {
	if !s.IsEnabled() {
		return "", fmt.Errorf("Ollama service is not available - ensure Ollama is running with %s model", s.model)
	}

	fmt.Printf("Generating response for user message: %s\n", userMessage)

	var contentBuilder strings.Builder

	if websiteContent != nil {
		//contentBuilder.WriteString("=== COMPREHENSIVE PROFILE ===\n\n")

		// Include main website content
		if websiteContent.Title != "" {
			contentBuilder.WriteString(fmt.Sprintf("MAIN WEBSITE: %s\n", websiteContent.Title))
		}
		if websiteContent.Description != "" {
			contentBuilder.WriteString(fmt.Sprintf("DESCRIPTION: %s\n", websiteContent.Description))
		}
		if websiteContent.Text != "" {
			contentBuilder.WriteString("MAIN WEBSITE CONTENT:\n")

			//content, err := s.SummarizeContent("main page", websiteContent.Text)
			//if err != nil {
			//	return "", fmt.Errorf("failed to summarize content: %v", err)
			//}
			//contentBuilder.WriteString(content)
			contentBuilder.WriteString(websiteContent.Text)

			contentBuilder.WriteString("\n\n")
		}

		// Include metadata
		if len(websiteContent.Metadata) > 0 {
			contentBuilder.WriteString("WEBSITE METADATA:\n")
			for key, value := range websiteContent.Metadata {
				contentBuilder.WriteString(fmt.Sprintf("- %s: %s\n", key, value))
			}
			contentBuilder.WriteString("\n")
		}

		//// Include all website links with descriptions
		//if len(websiteContent.Links) > 0 {
		//	contentBuilder.WriteString("PROFESSIONAL LINKS AND PROFILES:\n")
		//	for _, link := range websiteContent.Links {
		//		contentBuilder.WriteString(fmt.Sprintf("- %s: %s (Type: %s)\n", link.Title, link.URL, link.Type))
		//	}
		//	contentBuilder.WriteString("\n")
		//}

		// Include linked content from professional profiles
		if len(websiteContent.LinkedContent) > 0 {
			contentBuilder.WriteString("EXTERNAL PROFILE CONTENT:\n")
			for url, linkedContent := range websiteContent.LinkedContent {
				contentBuilder.WriteString(fmt.Sprintf("\n--- PROFILE: %s ---\n", url))
				if linkedContent.Title != "" {
					contentBuilder.WriteString(fmt.Sprintf("Title: %s\n", linkedContent.Title))
				}
				if linkedContent.Description != "" {
					contentBuilder.WriteString(fmt.Sprintf("Description: %s\n", linkedContent.Description))
				}
				if linkedContent.ContentType != "" {
					contentBuilder.WriteString(fmt.Sprintf("Content Type: %s\n", linkedContent.ContentType))
				}
				//if linkedContent.Relevance > 0 {
				//	contentBuilder.WriteString(fmt.Sprintf("Relevance Score: %d/10\n", linkedContent.Relevance))
				//}
				if len(linkedContent.Keywords) > 0 {
					contentBuilder.WriteString(fmt.Sprintf("Keywords: %s\n", strings.Join(linkedContent.Keywords, ", ")))
				}
				if linkedContent.Text != "" {
					contentBuilder.WriteString("Content:\n")

					//content, err := s.SummarizeContent(url, linkedContent.Text)
					//if err != nil {
					//	return "", fmt.Errorf("failed to summarize content: %v", err)
					//}
					//contentBuilder.WriteString(content)
					contentBuilder.WriteString(linkedContent.Text)

					contentBuilder.WriteString("\n")
				}

				// Include linked content
				if len(linkedContent.FirstLevelLinks) > 0 {
					contentBuilder.WriteString("FIRST-LEVEL LINKED CONTENT:\n")
					for _, firstLevel := range linkedContent.FirstLevelLinks {
						contentBuilder.WriteString(fmt.Sprintf("\n  • %s (%s)\n", firstLevel.Title, firstLevel.URL))
						if firstLevel.Description != "" {
							contentBuilder.WriteString(fmt.Sprintf("    Description: %s\n", firstLevel.Description))
						}
						if firstLevel.Relevance > 0 {
							contentBuilder.WriteString(fmt.Sprintf("    Relevance: %d/10\n", firstLevel.Relevance))
						}
						if firstLevel.Text != "" {
							//content, err := s.SummarizeContent(firstLevel.URL, firstLevel.Text)
							//if err != nil {
							//	return "", fmt.Errorf("failed to summarize content: %v", err)
							//}
							//contentBuilder.WriteString(fmt.Sprintf("    Content Summary: %s\n", content))
							contentBuilder.WriteString(fmt.Sprintf("    Content Summary: %s\n", firstLevel.Text))

						}
					}
					contentBuilder.WriteString("\n")
				}

				contentBuilder.WriteString("--- END PROFILE ---\n\n")
			}
		}

		// Include full PDF content (CV/Resume) for comprehensive analysis
		if len(websiteContent.PDFContent) > 0 {
			contentBuilder.WriteString("DETAILED CV/RESUME DOCUMENTS:\n")
			for url, pdf := range websiteContent.PDFContent {
				contentBuilder.WriteString(fmt.Sprintf("\n--- CV/RESUME FROM: %s ---\n", url))
				contentBuilder.WriteString(pdf.Text)
				contentBuilder.WriteString("\n--- END CV/RESUME ---\n\n")
			}
		}

		// Include parsed file content (XLSX, DOCX, CSV)
		if len(websiteContent.FileContent) > 0 {
			contentBuilder.WriteString("PARSED FILE DOCUMENTS:\n")
			for url, file := range websiteContent.FileContent {
				contentBuilder.WriteString(fmt.Sprintf("\n--- %s FILE FROM: %s ---\n", strings.ToUpper(file.FileType), url))
				contentBuilder.WriteString(fmt.Sprintf("File Name: %s\n", file.FileName))
				if len(file.SheetNames) > 0 {
					contentBuilder.WriteString(fmt.Sprintf("Sheets: %s\n", strings.Join(file.SheetNames, ", ")))
				}
				if file.RowCount > 0 {
					contentBuilder.WriteString(fmt.Sprintf("Rows: %d\n", file.RowCount))
				}
				if file.ColumnCount > 0 {
					contentBuilder.WriteString(fmt.Sprintf("Columns: %d\n", file.ColumnCount))
				}
				if len(file.Metadata) > 0 {
					contentBuilder.WriteString("Metadata:\n")
					for key, value := range file.Metadata {
						contentBuilder.WriteString(fmt.Sprintf("- %s: %s\n", key, value))
					}
				}
				contentBuilder.WriteString("Content:\n")

				contentBuilder.WriteString(file.Text)

				contentBuilder.WriteString(fmt.Sprintf("\n--- END %s FILE ---\n\n", strings.ToUpper(file.FileType)))
			}
		}
	}

	cb := contentBuilder.String()
	// Compile regex: one or more whitespace chars
	re := regexp.MustCompile(`\s+`)

	// Replace with single space
	cb = re.ReplaceAllString(cb, " ")

	// Limit content size to avoid overwhelming the AI TODO: configure
	if len(cb) > s.maxTotalContentLength {
		cb = cb[:s.maxTotalContentLength] + "..."
	}

	prompt := fmt.Sprintf(`You are an intelligent assistant with comprehensive information about this website. You have access to:
- Main website content and metadata
- Linked pages from external profiles with relevance scoring
- Parsed file documents (PDF, XLSX, DOCX, CSV) with structured data and metadata

COMPREHENSIVE DATA AVAILABLE:
%s

USER QUESTION: %s

INSTRUCTIONS:
1. Answer using information provided in this prompt only. Do not use external data. Do not make up answers."
2. Cross-reference information across datalinks for comprehensive answers
3. For file content (XLSX/DOCX/CSV/PDF), utilize structured data, metadata, and extracted information
4. Be conversational, detailed, and cite sources with their relevance when helpful
5. If information is limited, clearly state what's not available and suggest checking specific high-relevance sources
6. Think three times and provide the best possible answer.
7. Do not hallucinte or fabricate information.

Provide a thorough response.`, cb, userMessage)

	return s.generateResponse(prompt)
}

func (s *OllamaService) SummarizeContent(title, content string) (string, error) {
	if !s.IsEnabled() {
		return "", fmt.Errorf("Ollama service is not available - ensure Ollama is running with %s model", s.model)
	}

	fmt.Printf("Summarizing content for %s\n", title)

	if content == "" {
		return "", fmt.Errorf("no content provided")
	}

	// Compile regex: one or more whitespace chars
	re := regexp.MustCompile(`\s+`)

	// Replace with single space
	content = re.ReplaceAllString(content, " ")

	// Limit content size to avoid overwhelming the AI TODO: configure
	if len(content) > s.maxTotalContentLength {
		content = content[:s.maxTotalContentLength] + "..."
	}

	prompt := fmt.Sprintf(`You are an AI assistant analyzing a web page content. 

TITLE:
%s

CONTENT:
%s

INSTRUCTIONS:
1. Analyze the provided title and content
2. Provide relevant summary under 1000 characters based on the content
3. Preserve names, factual data, and numbers

Please provide an extended comprehensive summary based on the web page content above, to be used in further LLM analysis.`, title, content)

	return s.generateResponse(prompt)
}
