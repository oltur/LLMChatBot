package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tealeg/xlsx/v3"
	"github.com/unidoc/unioffice/document"
)

type FileParser struct {
	client *http.Client
}

type FileContent struct {
	Text        string
	FileName    string
	FileType    string
	SheetNames  []string
	RowCount    int
	ColumnCount int
	LastUpdated time.Time
	Metadata    map[string]string
}

func NewFileParser() *FileParser {
	return &FileParser{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (p *FileParser) ParseFromURL(fileURL string) (*FileContent, error) {
	resp, err := p.client.Get(fileURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch file from %s: %v", fileURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download file: status code %d", resp.StatusCode)
	}

	parsedURL, err := url.Parse(fileURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %v", err)
	}

	fileName := filepath.Base(parsedURL.Path)
	fileExt := strings.ToLower(filepath.Ext(fileName))

	switch fileExt {
	case ".xlsx":
		return p.parseXLSX(resp.Body, fileName)
	case ".docx":
		return p.parseDOCX(resp.Body, fileName)
	case ".csv":
		return p.parseCSV(resp.Body, fileName)
	default:
		return nil, fmt.Errorf("unsupported file type: %s", fileExt)
	}
}

func (p *FileParser) parseXLSX(reader io.Reader, fileName string) (*FileContent, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read XLSX data: %v", err)
	}

	wb, err := xlsx.OpenBinary(data)
	if err != nil {
		return nil, fmt.Errorf("failed to open XLSX file: %v", err)
	}

	content := &FileContent{
		FileName:    fileName,
		FileType:    "xlsx",
		LastUpdated: time.Now(),
		Metadata:    make(map[string]string),
	}

	var textBuilder strings.Builder
	var totalRows, totalCols int

	for _, sheet := range wb.Sheets {
		content.SheetNames = append(content.SheetNames, sheet.Name)
		textBuilder.WriteString(fmt.Sprintf("=== SHEET: %s ===\n", sheet.Name))

		maxRow := sheet.MaxRow
		maxCol := sheet.MaxCol
		totalRows += maxRow
		if maxCol > totalCols {
			totalCols = maxCol
		}

		for rowIndex := 0; rowIndex < maxRow; rowIndex++ {
			row, err := sheet.Row(rowIndex)
			if err != nil {
				continue
			}

			var rowData []string
			for colIndex := 0; colIndex < maxCol; colIndex++ {
				cell := row.GetCell(colIndex)
				if cell != nil {
					cellValue, _ := cell.FormattedValue()
					if strings.TrimSpace(cellValue) != "" {
						rowData = append(rowData, cellValue)
					}
				}
			}

			if len(rowData) > 0 {
				textBuilder.WriteString(strings.Join(rowData, " | "))
				textBuilder.WriteString("\n")
			}
		}
		textBuilder.WriteString("\n")
	}

	content.Text = textBuilder.String()
	content.RowCount = totalRows
	content.ColumnCount = totalCols
	content.Metadata["sheets_count"] = fmt.Sprintf("%d", len(wb.Sheets))

	return content, nil
}

func (p *FileParser) parseDOCX(reader io.Reader, fileName string) (*FileContent, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read DOCX data: %v", err)
	}

	tempFile := fmt.Sprintf("/tmp/temp_%d.docx", time.Now().UnixNano())
	err = os.WriteFile(tempFile, data, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile)

	doc, err := document.Open(tempFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open DOCX file: %v", err)
	}
	defer doc.Close()

	content := &FileContent{
		FileName:    fileName,
		FileType:    "docx",
		LastUpdated: time.Now(),
		Metadata:    make(map[string]string),
	}

	var textBuilder strings.Builder
	paragraphs := doc.Paragraphs()

	for _, para := range paragraphs {
		runs := para.Runs()
		var paraText strings.Builder

		for _, run := range runs {
			paraText.WriteString(run.Text())
		}

		paraTextStr := strings.TrimSpace(paraText.String())
		if paraTextStr != "" {
			textBuilder.WriteString(paraTextStr)
			textBuilder.WriteString("\n")
		}
	}

	props := doc.CoreProperties
	if title := props.Title(); title != "" {
		content.Metadata["title"] = title
	}

	content.Text = textBuilder.String()
	content.Metadata["paragraphs_count"] = fmt.Sprintf("%d", len(paragraphs))

	return content, nil
}

func (p *FileParser) parseCSV(reader io.Reader, fileName string) (*FileContent, error) {
	csvReader := csv.NewReader(reader)
	csvReader.FieldsPerRecord = -1

	content := &FileContent{
		FileName:    fileName,
		FileType:    "csv",
		LastUpdated: time.Now(),
		Metadata:    make(map[string]string),
	}

	var textBuilder strings.Builder
	var rowCount, maxCols int

	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading CSV: %v", err)
		}

		rowCount++
		if len(record) > maxCols {
			maxCols = len(record)
		}

		var cleanRecord []string
		for _, field := range record {
			field = strings.TrimSpace(field)
			if field != "" {
				cleanRecord = append(cleanRecord, field)
			}
		}

		if len(cleanRecord) > 0 {
			textBuilder.WriteString(strings.Join(cleanRecord, " | "))
			textBuilder.WriteString("\n")
		}
	}

	content.Text = textBuilder.String()
	content.RowCount = rowCount
	content.ColumnCount = maxCols
	content.Metadata["rows_count"] = fmt.Sprintf("%d", rowCount)
	content.Metadata["columns_count"] = fmt.Sprintf("%d", maxCols)

	return content, nil
}

func (p *FileParser) ExtractKeyInformation(content *FileContent) map[string]string {
	info := make(map[string]string)
	text := strings.ToLower(content.Text)

	for key, value := range content.Metadata {
		info[key] = value
	}

	info["file_type"] = content.FileType
	info["file_name"] = content.FileName

	if content.FileType == "xlsx" && len(content.SheetNames) > 0 {
		info["sheet_names"] = strings.Join(content.SheetNames, ", ")
	}

	if content.RowCount > 0 {
		info["row_count"] = fmt.Sprintf("%d", content.RowCount)
	}
	if content.ColumnCount > 0 {
		info["column_count"] = fmt.Sprintf("%d", content.ColumnCount)
	}

	skills := p.extractSkills(text)
	if len(skills) > 0 {
		info["detected_skills"] = strings.Join(skills, ", ")
	}

	dataTypes := p.detectDataTypes(text)
	if len(dataTypes) > 0 {
		info["data_types"] = strings.Join(dataTypes, ", ")
	}

	return info
}

func (p *FileParser) extractSkills(text string) []string {
	var skills []string
	skillKeywords := []string{
		"golang", "go", "python", "javascript", "typescript", "java", "c++", "c#", "rust",
		"docker", "kubernetes", "aws", "azure", "gcp", "linux", "git", "sql", "nosql",
		"react", "vue", "angular", "node.js", "express", "django", "flask", "spring",
		"microservices", "api", "rest", "graphql", "mongodb", "postgresql", "mysql",
		"redis", "elasticsearch", "kafka", "rabbitmq", "terraform", "ansible",
		"jenkins", "github actions", "ci/cd", "devops", "machine learning", "ai",
		"blockchain", "tensorflow", "pytorch", "opencv", "pandas", "numpy",
		"excel", "powerbi", "tableau", "analytics", "data science", "statistics",
	}

	for _, skill := range skillKeywords {
		if strings.Contains(text, skill) {
			skills = append(skills, skill)
		}
	}

	return skills
}

func (p *FileParser) detectDataTypes(text string) []string {
	var dataTypes []string

	if strings.Contains(text, "email") || strings.Contains(text, "@") {
		dataTypes = append(dataTypes, "email")
	}
	if strings.Contains(text, "phone") || strings.Contains(text, "tel") {
		dataTypes = append(dataTypes, "phone")
	}
	if strings.Contains(text, "address") || strings.Contains(text, "street") {
		dataTypes = append(dataTypes, "address")
	}
	if strings.Contains(text, "date") || strings.Contains(text, "/") {
		dataTypes = append(dataTypes, "date")
	}
	if strings.Contains(text, "$") || strings.Contains(text, "price") || strings.Contains(text, "cost") {
		dataTypes = append(dataTypes, "financial")
	}
	if strings.Contains(text, "project") || strings.Contains(text, "task") {
		dataTypes = append(dataTypes, "project_data")
	}
	if strings.Contains(text, "resume") || strings.Contains(text, "cv") || strings.Contains(text, "experience") {
		dataTypes = append(dataTypes, "resume_data")
	}

	return dataTypes
}

func (p *FileParser) isValidFileURL(rawURL string) bool {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	path := strings.ToLower(parsedURL.Path)
	return strings.HasSuffix(path, ".xlsx") ||
		strings.HasSuffix(path, ".docx") ||
		strings.HasSuffix(path, ".csv")
}
