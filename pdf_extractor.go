package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
)

type PDFExtractor struct {
	client *http.Client
}

type PDFContent struct {
	Text        string
	Pages       int
	Title       string
	Author      string
	Subject     string
	Keywords    string
	LastUpdated time.Time
}

func NewPDFExtractor() *PDFExtractor {
	return &PDFExtractor{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (p *PDFExtractor) ExtractFromURL(pdfURL string) (*PDFContent, error) {
	resp, err := p.client.Get(pdfURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch PDF from %s: %v", pdfURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download PDF: status code %d", resp.StatusCode)
	}

	return p.extractFromReader(resp.Body)
}

func (p *PDFExtractor) extractFromReader(reader io.Reader) (*PDFContent, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read PDF data: %v", err)
	}

	pdfReader, err := pdf.NewReader(strings.NewReader(string(data)), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to create PDF reader: %v", err)
	}

	content := &PDFContent{
		Pages:       pdfReader.NumPage(),
		LastUpdated: time.Now(),
	}

	var textContent strings.Builder
	for i := 1; i <= content.Pages; i++ {
		page := pdfReader.Page(i)
		if page.V.IsNull() {
			continue
		}

		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}

		textContent.WriteString(text)
		textContent.WriteString("\n")
	}

	content.Text = strings.TrimSpace(textContent.String())
	return content, nil
}

func (p *PDFExtractor) ExtractKeyInformation(content *PDFContent) map[string]string {
	info := make(map[string]string)
	text := strings.ToLower(content.Text)

	info["title"] = content.Title
	info["author"] = content.Author
	info["subject"] = content.Subject
	info["keywords"] = content.Keywords
	info["pages"] = fmt.Sprintf("%d", content.Pages)

	skills := p.extractSkills(text)
	if len(skills) > 0 {
		info["skills"] = strings.Join(skills, ", ")
	}

	experience := p.extractExperience(text)
	if len(experience) > 0 {
		info["experience"] = strings.Join(experience, "; ")
	}

	education := p.extractEducation(text)
	if len(education) > 0 {
		info["education"] = strings.Join(education, "; ")
	}

	contact := p.extractContactInfo(text)
	if len(contact) > 0 {
		info["contact"] = strings.Join(contact, ", ")
	}

	return info
}

func (p *PDFExtractor) extractSkills(text string) []string {
	var skills []string
	skillKeywords := []string{
		"golang", "go", "python", "javascript", "typescript", "java", "c++", "c#", "rust",
		"docker", "kubernetes", "aws", "azure", "gcp", "linux", "git", "sql", "nosql",
		"react", "vue", "angular", "node.js", "express", "django", "flask", "spring",
		"microservices", "api", "rest", "graphql", "mongodb", "postgresql", "mysql",
		"redis", "elasticsearch", "kafka", "rabbitmq", "terraform", "ansible",
		"jenkins", "github actions", "ci/cd", "devops", "machine learning", "ai",
		"blockchain", "tensorflow", "pytorch", "opencv", "pandas", "numpy",
	}

	for _, skill := range skillKeywords {
		if strings.Contains(text, skill) {
			skills = append(skills, skill)
		}
	}

	return skills
}

func (p *PDFExtractor) extractExperience(text string) []string {
	var experience []string
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) < 20 {
			continue
		}

		lower := strings.ToLower(line)
		if strings.Contains(lower, "experience") ||
			strings.Contains(lower, "worked") ||
			strings.Contains(lower, "developer") ||
			strings.Contains(lower, "engineer") ||
			strings.Contains(lower, "years") {
			if len(experience) < 5 {
				experience = append(experience, line)
			}
		}
	}

	return experience
}

func (p *PDFExtractor) extractEducation(text string) []string {
	var education []string
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) < 10 {
			continue
		}

		lower := strings.ToLower(line)
		if strings.Contains(lower, "education") ||
			strings.Contains(lower, "university") ||
			strings.Contains(lower, "degree") ||
			strings.Contains(lower, "bachelor") ||
			strings.Contains(lower, "master") ||
			strings.Contains(lower, "phd") {
			if len(education) < 3 {
				education = append(education, line)
			}
		}
	}

	return education
}

func (p *PDFExtractor) extractContactInfo(text string) []string {
	var contact []string
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)

		if strings.Contains(line, "@") && strings.Contains(line, ".") {
			contact = append(contact, "Email: "+line)
		}

		if strings.Contains(lower, "phone") || strings.Contains(lower, "tel") {
			contact = append(contact, line)
		}
	}

	return contact
}

func (p *PDFExtractor) isValidPDFURL(rawURL string) bool {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	path := strings.ToLower(parsedURL.Path)
	return strings.HasSuffix(path, ".pdf")
}
