// Package tools provides agent tool implementations.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// WebSearchTool searches the web using Brave Search API.
type WebSearchTool struct {
	// APIKey is the Brave Search API key.
	APIKey string
	// DefaultCount is the default number of results.
	DefaultCount int
	// Timeout is the request timeout.
	Timeout time.Duration
}

// NewWebSearchTool creates a new web search tool.
func NewWebSearchTool() *WebSearchTool {
	apiKey := os.Getenv("BRAVE_API_KEY")
	return &WebSearchTool{
		APIKey:       apiKey,
		DefaultCount: 5,
		Timeout:      30 * time.Second,
	}
}

// Name returns the tool name.
func (t *WebSearchTool) Name() string {
	return "web_search"
}

// Description returns the tool description.
func (t *WebSearchTool) Description() string {
	return `Search the web using Brave Search API.
Supports region-specific and localized search via country and language parameters.
Returns titles, URLs, and snippets for fast research.`
}

// Parameters returns the JSON Schema for parameters.
func (t *WebSearchTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query string",
			},
			"count": map[string]interface{}{
				"type":        "integer",
				"description": "Number of results to return (1-10, default: 5)",
				"minimum":     1,
				"maximum":     10,
			},
			"country": map[string]interface{}{
				"type":        "string",
				"description": "2-letter country code for region-specific results (e.g., 'US', 'DE')",
			},
			"search_lang": map[string]interface{}{
				"type":        "string",
				"description": "ISO language code for search results (e.g., 'en', 'de', 'jp')",
			},
			"ui_lang": map[string]interface{}{
				"type":        "string",
				"description": "ISO language code for UI elements",
			},
			"freshness": map[string]interface{}{
				"type":        "string",
				"description": "Filter by time: pd (24h), pw (week), pm (month), py (year)",
			},
		},
		"required": []string{"query"},
	}
}

// SearchResult represents a single search result.
type SearchResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Published   string `json:"published,omitempty"`
	SiteName    string `json:"siteName,omitempty"`
}

// WebSearchResult represents the search results.
type WebSearchResult struct {
	Query   string         `json:"query"`
	Count   int            `json:"count"`
	Results []SearchResult `json:"results"`
	TookMs  int64          `json:"tookMs"`
}

// BraveSearchResponse represents the Brave Search API response.
type BraveSearchResponse struct {
	Web struct {
		Results []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
			Age         string `json:"age"`
		} `json:"results"`
	} `json:"web"`
}

// Execute performs the web search.
func (t *WebSearchTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	query, _ := params["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	if t.APIKey == "" {
		return nil, fmt.Errorf("BRAVE_API_KEY environment variable is not set")
	}

	count := t.DefaultCount
	if c, ok := params["count"].(float64); ok && c > 0 {
		count = int(c)
		if count > 10 {
			count = 10
		}
	}

	country, _ := params["country"].(string)
	freshness, _ := params["freshness"].(string)
	searchLang, _ := params["search_lang"].(string)
	uiLang, _ := params["ui_lang"].(string)

	start := time.Now()

	// Build URL
	searchURL, _ := url.Parse("https://api.search.brave.com/res/v1/web/search")
	q := searchURL.Query()
	q.Set("q", query)
	q.Set("count", fmt.Sprintf("%d", count))
	if country != "" {
		q.Set("country", country)
	}
	if freshness != "" {
		q.Set("freshness", freshness)
	}
	if searchLang != "" {
		q.Set("search_lang", searchLang)
	}
	if uiLang != "" {
		q.Set("ui_lang", uiLang)
	}
	searchURL.RawQuery = q.Encode()

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", t.APIKey)

	// Execute request
	client := &http.Client{Timeout: t.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search API error (%d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var braveResp BraveSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&braveResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Map results
	results := make([]SearchResult, 0, len(braveResp.Web.Results))
	for _, r := range braveResp.Web.Results {
		var siteName string
		if u, err := url.Parse(r.URL); err == nil {
			siteName = u.Hostname()
		}

		results = append(results, SearchResult{
			Title:       r.Title,
			URL:         r.URL,
			Description: r.Description,
			Published:   r.Age,
			SiteName:    siteName,
		})
	}

	return &WebSearchResult{
		Query:   query,
		Count:   len(results),
		Results: results,
		TookMs:  time.Since(start).Milliseconds(),
	}, nil
}

// WebFetchTool fetches and extracts content from URLs.
type WebFetchTool struct {
	// Timeout is the request timeout.
	Timeout time.Duration
	// MaxChars is the maximum characters to return.
	MaxChars int
	// UserAgent is the User-Agent header.
	UserAgent string
}

// NewWebFetchTool creates a new web fetch tool.
func NewWebFetchTool() *WebFetchTool {
	return &WebFetchTool{
		Timeout:   30 * time.Second,
		MaxChars:  50000,
		UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_7_2) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	}
}

// Name returns the tool name.
func (t *WebFetchTool) Name() string {
	return "web_fetch"
}

// Description returns the tool description.
func (t *WebFetchTool) Description() string {
	return `Fetch and extract readable content from a URL.
Converts HTML to text/markdown for easy processing.
Use for lightweight page access without browser automation.`
}

// Parameters returns the JSON Schema for parameters.
func (t *WebFetchTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "HTTP or HTTPS URL to fetch",
			},
			"maxChars": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum characters to return (default: 50000)",
			},
			"extractMode": map[string]interface{}{
				"type":        "string",
				"description": "Extraction mode: 'text' or 'markdown' (default: markdown)",
				"enum":        []string{"text", "markdown"},
			},
		},
		"required": []string{"url"},
	}
}

// WebFetchResult represents the fetch result.
type WebFetchResult struct {
	URL         string `json:"url"`
	FinalURL    string `json:"finalUrl"`
	Status      int    `json:"status"`
	ContentType string `json:"contentType"`
	Length      int    `json:"length"`
	Truncated   bool   `json:"truncated"`
	TookMs      int64  `json:"tookMs"`
	Text        string `json:"text"`
}

// Execute fetches the URL content.
func (t *WebFetchTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	urlStr, _ := params["url"].(string)
	if urlStr == "" {
		return nil, fmt.Errorf("url is required")
	}

	// Validate URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("invalid URL: must be http or https")
	}

	maxChars := t.MaxChars
	if mc, ok := params["maxChars"].(float64); ok && mc > 0 {
		maxChars = int(mc)
	}

	start := time.Now()

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", t.UserAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	// Execute request
	client := &http.Client{
		Timeout: t.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		// Assuming truncateForDiff is available in tool package or using simple string limit
		return nil, fmt.Errorf("fetch failed (%d): %s", resp.StatusCode, string(body))
	}

	// Read body with limit
	limitReader := io.LimitReader(resp.Body, int64(maxChars*2))
	body, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	text := string(body)
	contentType := resp.Header.Get("Content-Type")

	// Simple HTML to text extraction
	if strings.Contains(contentType, "text/html") {
		text = extractTextFromHTML(text)
	}

	truncated := false
	if len(text) > maxChars {
		text = text[:maxChars]
		truncated = true
	}

	return &WebFetchResult{
		URL:         urlStr,
		FinalURL:    resp.Request.URL.String(),
		Status:      resp.StatusCode,
		ContentType: contentType,
		Length:      len(text),
		Truncated:   truncated,
		TookMs:      time.Since(start).Milliseconds(),
		Text:        text,
	}, nil
}

// extractTextFromHTML performs a basic HTML to text extraction.
func extractTextFromHTML(html string) string {
	// Remove script and style tags
	html = removeTagContent(html, "script")
	html = removeTagContent(html, "style")

	// Remove all HTML tags
	var result strings.Builder
	inTag := false
	for _, r := range html {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			result.WriteRune(' ')
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}

	// Clean up whitespace
	text := result.String()
	text = strings.ReplaceAll(text, "\t", " ")
	text = strings.ReplaceAll(text, "\r", "")

	// Collapse multiple newlines
	lines := strings.Split(text, "\n")
	var cleanLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleanLines = append(cleanLines, line)
		}
	}

	return strings.Join(cleanLines, "\n")
}

// removeTagContent removes content between opening and closing tags.
func removeTagContent(html, tag string) string {
	openTag := "<" + tag
	closeTag := "</" + tag + ">"

	result := html
	for {
		start := strings.Index(strings.ToLower(result), openTag)
		if start == -1 {
			break
		}
		end := strings.Index(strings.ToLower(result[start:]), closeTag)
		if end == -1 {
			break
		}
		result = result[:start] + result[start+end+len(closeTag):]
	}
	return result
}
