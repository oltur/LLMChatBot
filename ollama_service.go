package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type OllamaService struct {
	baseURL string
	model   string
	client  *http.Client
}

type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
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

	return &OllamaService{
		baseURL: baseURL,
		model:   model,
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
		return "", fmt.Errorf("Ollama API error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Ollama API returned status code: %d", resp.StatusCode)
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

	prompt := fmt.Sprintf(`You are an AI assistant analyzing Oleksandr Turevskiy's CV/Resume. 

CV Content:
%s

User Question: %s

Please analyze the CV content and provide a comprehensive answer. Focus on extracting relevant information about skills, experience, education, and achievements.`, content, question)

	return s.generateResponse(prompt)
}

func (s *OllamaService) GenerateIntelligentResponse(websiteContent *WebsiteContent, userMessage string) (string, error) {
	if !s.IsEnabled() {
		return "", fmt.Errorf("Ollama service is not available - ensure Ollama is running with %s model", s.model)
	}

	var contentBuilder strings.Builder

	if websiteContent != nil {
		contentBuilder.WriteString("=== COMPREHENSIVE OLEKSANDR TUREVSKIY PROFILE ===\n\n")

		// Include main website content
		if websiteContent.Title != "" {
			contentBuilder.WriteString(fmt.Sprintf("MAIN WEBSITE: %s\n", websiteContent.Title))
		}
		if websiteContent.Description != "" {
			contentBuilder.WriteString(fmt.Sprintf("DESCRIPTION: %s\n", websiteContent.Description))
		}
		if websiteContent.Text != "" {
			contentBuilder.WriteString("MAIN WEBSITE CONTENT:\n")
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

		// Include all website links with descriptions
		if len(websiteContent.Links) > 0 {
			contentBuilder.WriteString("PROFESSIONAL LINKS AND PROFILES:\n")
			for _, link := range websiteContent.Links {
				contentBuilder.WriteString(fmt.Sprintf("- %s: %s (Type: %s)\n", link.Title, link.URL, link.Type))
			}
			contentBuilder.WriteString("\n")
		}

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
				if linkedContent.Relevance > 0 {
					contentBuilder.WriteString(fmt.Sprintf("Relevance Score: %d/10\n", linkedContent.Relevance))
				}
				if len(linkedContent.Keywords) > 0 {
					contentBuilder.WriteString(fmt.Sprintf("Keywords: %s\n", strings.Join(linkedContent.Keywords, ", ")))
				}
				if linkedContent.Text != "" {
					contentBuilder.WriteString("Content:\n")
					contentBuilder.WriteString(linkedContent.Text)
					contentBuilder.WriteString("\n")
				}

				// Include first-level linked content
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
	}

	prompt := fmt.Sprintf(`You are an intelligent assistant with comprehensive information about Oleksandr Turevskiy. You have access to:
- His main website content and metadata
- Full CV/resume documents with detailed professional information
- Content from external professional profiles (GitHub, LinkedIn, etc.)
- First-level linked pages from external profiles with relevance scoring
- All professional links and social profiles
- Complete biographical and career information with content type classification

COMPREHENSIVE DATA AVAILABLE:
%s

USER QUESTION: %s

INSTRUCTIONS:
1. Answer using ALL available information including main website, external profiles, CV documents, and first-level linked content
2. Provide specific details from any relevant source, considering relevance scores (higher scores = more reliable)
3. Cross-reference information across sources and first-level links for comprehensive answers
4. For skills/experience questions, use CV content, profile information, AND linked project pages
5. For contact/social info, reference the professional links and their content types
6. For projects/code, utilize GitHub/GitLab profiles AND their first-level linked repositories/projects
7. Pay attention to content types (professional, blog, project, technical) when providing answers
8. Be conversational, detailed, and cite sources with their relevance when helpful
9. Use first-level linked content to provide deeper insights into projects, articles, and professional work
10. If information is limited, clearly state what's not available and suggest checking specific high-relevance sources

Provide a thorough response using the comprehensive data available above.`, contentBuilder.String(), userMessage)

	return s.generateResponse(prompt)
}
