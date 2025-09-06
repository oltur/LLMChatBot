package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Chatbot struct {
	scraper       *WebScraper
	ollamaService *OllamaService
	websiteURL    string
	websiteData   *WebsiteContent
	lastDataFetch time.Time
}

type ChatMessage struct {
	Message   string    `json:"message"`
	Response  string    `json:"response"`
	Timestamp time.Time `json:"timestamp"`
}

func NewChatbot(scraper *WebScraper, ollamaService *OllamaService) *Chatbot {
	websiteURL := os.Getenv("WEBSITE_URL")
	// Note: WEBSITE_URL validation is handled in main()

	return &Chatbot{
		scraper:       scraper,
		ollamaService: ollamaService,
		websiteURL:    websiteURL,
	}
}

func (c *Chatbot) refreshWebsiteData() error {
	if c.websiteData != nil && time.Since(c.lastDataFetch) < 1*time.Hour {
		return nil
	}

	// Clear previous scraping logs for a fresh session
	c.scraper.ClearScrapedUrls()

	data, err := c.scraper.ScrapeWebsite(c.websiteURL)
	if err != nil {
		return fmt.Errorf("failed to refresh website data: %v", err)
	}

	// Print scraping summary after successful scraping
	c.scraper.PrintScrapedUrls()

	c.websiteData = data
	c.lastDataFetch = time.Now()
	return nil
}

func (c *Chatbot) ProcessMessage(message string) (*ChatMessage, error) {
	if err := c.refreshWebsiteData(); err != nil {
		return nil, err
	}

	response := c.generateResponse(message)

	return &ChatMessage{
		Message:   message,
		Response:  response,
		Timestamp: time.Now(),
	}, nil
}

func (c *Chatbot) generateResponse(message string) string {
	// Always try to use Ollama first with all available content
	if c.ollamaService != nil && c.ollamaService.IsEnabled() {
		response, err := c.ollamaService.GenerateIntelligentResponse(c.websiteData, message)
		if err == nil {
			return response
		}
		fmt.Printf("Ollama service error: %v\n", err)
	}

	// Fallback to rule-based responses only if Ollama is not available
	return c.getRuleBasedResponse(message)
}

func (c *Chatbot) getRuleBasedResponse(message string) string {
	lowerMsg := strings.ToLower(message)

	if strings.Contains(lowerMsg, "hello") || strings.Contains(lowerMsg, "hi ") || lowerMsg == "hi" {
		return "Hello! I'm here to help you learn about John Smith. You can ask me about his background, professional profiles, or any information available on his website."
	}

	if strings.Contains(lowerMsg, "who") && (strings.Contains(lowerMsg, "Somebody") || strings.Contains(lowerMsg, "turevskiy")) {
		return c.getPersonInfo()
	}

	if strings.Contains(lowerMsg, "contact") || strings.Contains(lowerMsg, "reach") {
		return c.getContactInfo()
	}

	if strings.Contains(lowerMsg, "github") || strings.Contains(lowerMsg, "code") || strings.Contains(lowerMsg, "projects") {
		return c.getGitHubInfo()
	}

	if strings.Contains(lowerMsg, "linkedin") || strings.Contains(lowerMsg, "professional") || strings.Contains(lowerMsg, "career") {
		return c.getLinkedInInfo()
	}

	if strings.Contains(lowerMsg, "blog") || strings.Contains(lowerMsg, "writing") {
		return c.getBlogInfo()
	}

	if strings.Contains(lowerMsg, "vitae") || strings.Contains(lowerMsg, "cv") || strings.Contains(lowerMsg, "resume") {
		return c.getCVInfo()
	}

	if strings.Contains(lowerMsg, "skills") || strings.Contains(lowerMsg, "technologies") || strings.Contains(lowerMsg, "programming") {
		return c.getSkillsInfo()
	}

	if strings.Contains(lowerMsg, "experience") || strings.Contains(lowerMsg, "work") || strings.Contains(lowerMsg, "job") {
		return c.getExperienceInfo()
	}

	if strings.Contains(lowerMsg, "education") || strings.Contains(lowerMsg, "degree") || strings.Contains(lowerMsg, "university") {
		return c.getEducationInfo()
	}

	if strings.Contains(lowerMsg, "help") || strings.Contains(lowerMsg, "what can you") {
		return c.getHelpInfo()
	}

	return `I'm specialized in providing information about John Smith based on his website content. 

You can ask me about:
• His professional background
• GitHub projects and code
• LinkedIn profile
• Professional blog
• CV/Resume
• How to contact him

Is there something specific you'd like to know about Somebody?`
}

func (c *Chatbot) getPersonInfo() string {
	if c.websiteData == nil {
		return "I'm having trouble accessing the website data right now. Please try again in a moment."
	}

	return "John Smith appears to be a software professional based on his online presence. His website provides links to various professional profiles including GitHub, GitLab, LinkedIn, and his professional blog. You can ask me about any of these specific areas for more information."
}

func (c *Chatbot) getContactInfo() string {
	links := c.getProfileLinks()
	if len(links) == 0 {
		return "I found several ways to connect with Somebody: through his GitHub, GitLab, LinkedIn profiles, or his professional blog."
	}

	response := "Here are the ways to connect with John Smith:\n"
	for _, link := range links {
		response += fmt.Sprintf("• %s: %s\n", link.Title, link.URL)
	}
	return response
}

func (c *Chatbot) getGitHubInfo() string {
	github := c.findLinkByKeyword("github")
	if github != nil {
		return fmt.Sprintf("You can find Somebody's code and projects on his GitHub profile: %s", github.URL)
	}
	return "Somebody has a GitHub profile where you can explore his code and projects."
}

func (c *Chatbot) getLinkedInInfo() string {
	linkedin := c.findLinkByKeyword("linkedin")
	if linkedin != nil {
		return fmt.Sprintf("For professional information about Somebody, check his LinkedIn profile: %s", linkedin.URL)
	}
	return "Somebody maintains a LinkedIn profile with his professional information."
}

func (c *Chatbot) getBlogInfo() string {
	blog := c.findLinkByKeyword("blog")
	if blog != nil {
		return fmt.Sprintf("Somebody shares his thoughts and expertise on his professional blog: %s", blog.URL)
	}
	return "Somebody has a professional blog where he shares insights and expertise."
}

func (c *Chatbot) getCVInfo() string {
	cv := c.findLinkByKeyword("cv")
	if cv != nil {
		response := fmt.Sprintf("You can view Somebody's CV/Resume here: %s", cv.URL)

		if c.websiteData != nil && c.websiteData.PDFContent != nil {
			if pdfContent, exists := c.websiteData.PDFContent[cv.URL]; exists {
				if c.ollamaService != nil && c.ollamaService.IsEnabled() {
					aiAnalysis, err := c.ollamaService.AnalyzePDFContent(pdfContent, "Provide a comprehensive summary of this CV including key skills, experience, and qualifications.")
					if err == nil {
						response += "\n\nAI Analysis of the CV:\n" + aiAnalysis
						return response
					}
				}

				keyInfo := c.extractPDFKeyInfo(pdfContent)
				if keyInfo != "" {
					response += "\n\nKey information from the CV:\n" + keyInfo
				}
			}
		}

		return response
	}
	return "Somebody's CV/Resume is available on his website."
}

func (c *Chatbot) getHelpInfo() string {
	aiStatus := ""
	if c.ollamaService != nil && c.ollamaService.IsEnabled() {
		aiStatus = " (Enhanced with CodeLlama AI analysis)"
	}

	return fmt.Sprintf(`I can help you learn about John Smith! Here's what you can ask me about%s:

• Personal and professional background
• GitHub profile and projects
• LinkedIn professional information  
• Professional blog and writings
• CV/Resume details (with AI-powered PDF content analysis)
• Technical skills and technologies
• Work experience and career history
• Educational background
• Contact information
• GitLab profile

I can analyze PDF documents (like his CV) using advanced AI to provide detailed insights about skills, experience, and education. You can also ask me general questions and I'll provide intelligent responses based on all available website content.

Just ask me anything about these topics!`, aiStatus)
}

func (c *Chatbot) findLinkByKeyword(keyword string) *Link {
	if c.websiteData == nil {
		return nil
	}

	for _, link := range c.websiteData.Links {
		if strings.Contains(strings.ToLower(link.URL), keyword) ||
			strings.Contains(strings.ToLower(link.Title), keyword) {
			return &link
		}
	}
	return nil
}

func (c *Chatbot) getProfileLinks() []Link {
	if c.websiteData == nil {
		return []Link{}
	}

	var profileLinks []Link
	keywords := []string{"github", "gitlab", "linkedin", "blog", "cv"}

	for _, keyword := range keywords {
		if link := c.findLinkByKeyword(keyword); link != nil {
			profileLinks = append(profileLinks, *link)
		}
	}

	return profileLinks
}

func (c *Chatbot) extractPDFKeyInfo(pdfContent *PDFContent) string {
	if pdfContent == nil {
		return ""
	}

	extractor := NewPDFExtractor()
	keyInfo := extractor.ExtractKeyInformation(pdfContent)

	var result []string

	if skills, exists := keyInfo["skills"]; exists && skills != "" {
		result = append(result, "Skills: "+skills)
	}

	if experience, exists := keyInfo["experience"]; exists && experience != "" {
		lines := strings.Split(experience, ";")
		if len(lines) > 0 {
			result = append(result, "Experience: "+strings.TrimSpace(lines[0]))
		}
	}

	if education, exists := keyInfo["education"]; exists && education != "" {
		lines := strings.Split(education, ";")
		if len(lines) > 0 {
			result = append(result, "Education: "+strings.TrimSpace(lines[0]))
		}
	}

	return strings.Join(result, "\n")
}

func (c *Chatbot) getSkillsInfo() string {
	if c.websiteData != nil && c.websiteData.PDFContent != nil {
		for _, pdfContent := range c.websiteData.PDFContent {
			if c.ollamaService != nil && c.ollamaService.IsEnabled() {
				aiAnalysis, err := c.ollamaService.AnalyzePDFContent(pdfContent, "Extract and analyze all technical skills, programming languages, frameworks, and technologies mentioned in this CV. Organize them by category.")
				if err == nil {
					return fmt.Sprintf("AI Analysis of Somebody's Technical Skills:\n%s\n\nFor more details, check his CV and GitHub profile.", aiAnalysis)
				}
			}

			extractor := NewPDFExtractor()
			keyInfo := extractor.ExtractKeyInformation(pdfContent)

			if skills, exists := keyInfo["skills"]; exists && skills != "" {
				return fmt.Sprintf("Based on Somebody's CV, here are his technical skills:\n%s\n\nFor more details, check his CV and GitHub profile.", skills)
			}
		}
	}

	return "You can find information about Somebody's technical skills in his CV and by exploring his GitHub projects. His GitHub profile showcases his practical experience with various technologies."
}

func (c *Chatbot) getExperienceInfo() string {
	if c.websiteData != nil && c.websiteData.PDFContent != nil {
		for _, pdfContent := range c.websiteData.PDFContent {
			if c.ollamaService != nil && c.ollamaService.IsEnabled() {
				aiAnalysis, err := c.ollamaService.AnalyzePDFContent(pdfContent, "Analyze and summarize the professional work experience, including companies, roles, responsibilities, and key achievements. Focus on career progression and impact.")
				if err == nil {
					return fmt.Sprintf("AI Analysis of Somebody's Professional Experience:\n%s\n\nFor complete work history, please check his full CV and LinkedIn profile.", aiAnalysis)
				}
			}

			extractor := NewPDFExtractor()
			keyInfo := extractor.ExtractKeyInformation(pdfContent)

			if experience, exists := keyInfo["experience"]; exists && experience != "" {
				experienceItems := strings.Split(experience, ";")
				if len(experienceItems) > 0 {
					return fmt.Sprintf("Here's information about Somebody's professional experience:\n\n%s\n\nFor complete work history, please check his full CV and LinkedIn profile.", strings.Join(experienceItems[:minInt(3, len(experienceItems))], "\n\n"))
				}
			}
		}
	}

	return "You can find detailed information about Somebody's work experience in his CV and LinkedIn profile. His GitHub and GitLab profiles also showcase his project experience."
}

func (c *Chatbot) getEducationInfo() string {
	if c.websiteData != nil && c.websiteData.PDFContent != nil {
		for _, pdfContent := range c.websiteData.PDFContent {
			if c.ollamaService != nil && c.ollamaService.IsEnabled() {
				aiAnalysis, err := c.ollamaService.AnalyzePDFContent(pdfContent, "Extract and analyze educational background including degrees, institutions, graduation dates, academic achievements, and relevant coursework.")
				if err == nil {
					return fmt.Sprintf("AI Analysis of Somebody's Educational Background:\n%s\n\nFor more details, check his full CV.", aiAnalysis)
				}
			}

			extractor := NewPDFExtractor()
			keyInfo := extractor.ExtractKeyInformation(pdfContent)

			if education, exists := keyInfo["education"]; exists && education != "" {
				educationItems := strings.Split(education, ";")
				return fmt.Sprintf("Here's information about Somebody's educational background:\n\n%s\n\nFor more details, check his full CV.", strings.Join(educationItems, "\n"))
			}
		}
	}

	return "Information about Somebody's educational background can be found in his CV/Resume. Please check the CV link for complete academic details."
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
