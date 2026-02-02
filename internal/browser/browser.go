// Package browser provides browser automation using CDP.
package browser

import (
	"context"
	"time"

	"github.com/chromedp/chromedp"
)

// Browser manages browser automation.
type Browser struct {
	ctx        context.Context
	cancel     context.CancelFunc
	controlURL string
	connected  bool
}

// Config holds browser configuration.
type Config struct {
	ControlURL string // CDP endpoint, e.g., http://localhost:9222
	Headless   bool
	Timeout    time.Duration
}

// New creates a new browser instance.
func New(cfg *Config) *Browser {
	return &Browser{
		controlURL: cfg.ControlURL,
	}
}

// Connect connects to the browser.
func (b *Browser) Connect(ctx context.Context) error {
	allocCtx, cancel := chromedp.NewRemoteAllocator(ctx, b.controlURL)
	b.cancel = cancel

	taskCtx, taskCancel := chromedp.NewContext(allocCtx)
	b.ctx = taskCtx

	// Test connection
	if err := chromedp.Run(taskCtx, chromedp.Navigate("about:blank")); err != nil {
		taskCancel()
		cancel()
		return err
	}

	b.connected = true
	return nil
}

// Close closes the browser connection.
func (b *Browser) Close() {
	if b.cancel != nil {
		b.cancel()
	}
	b.connected = false
}

// IsConnected returns whether the browser is connected.
func (b *Browser) IsConnected() bool {
	return b.connected
}

// Navigate navigates to a URL.
func (b *Browser) Navigate(ctx context.Context, url string) error {
	return chromedp.Run(b.ctx, chromedp.Navigate(url))
}

// GetContent gets the page content.
func (b *Browser) GetContent(ctx context.Context) (string, error) {
	var content string
	err := chromedp.Run(b.ctx, chromedp.InnerHTML("html", &content))
	return content, err
}

// Screenshot takes a screenshot.
func (b *Browser) Screenshot(ctx context.Context, fullPage bool) ([]byte, error) {
	var buf []byte
	var action chromedp.Action

	if fullPage {
		action = chromedp.FullScreenshot(&buf, 90)
	} else {
		action = chromedp.CaptureScreenshot(&buf)
	}

	if err := chromedp.Run(b.ctx, action); err != nil {
		return nil, err
	}

	return buf, nil
}

// Click clicks on an element.
func (b *Browser) Click(ctx context.Context, selector string) error {
	return chromedp.Run(b.ctx,
		chromedp.Click(selector, chromedp.ByQuery),
	)
}

// Type types text into an element.
func (b *Browser) Type(ctx context.Context, selector, text string) error {
	return chromedp.Run(b.ctx,
		chromedp.SendKeys(selector, text, chromedp.ByQuery),
	)
}

// WaitVisible waits for an element to be visible.
func (b *Browser) WaitVisible(ctx context.Context, selector string, timeout time.Duration) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_ = timeoutCtx // use the timeout context for future chromedp operations if needed

	return chromedp.Run(b.ctx,
		chromedp.WaitVisible(selector, chromedp.ByQuery),
	)
}

// Evaluate evaluates JavaScript in the page.
func (b *Browser) Evaluate(ctx context.Context, js string) (interface{}, error) {
	var result interface{}
	err := chromedp.Run(b.ctx, chromedp.Evaluate(js, &result))
	return result, err
}

// GetText gets the text content of an element.
func (b *Browser) GetText(ctx context.Context, selector string) (string, error) {
	var text string
	err := chromedp.Run(b.ctx, chromedp.Text(selector, &text, chromedp.ByQuery))
	return text, err
}

// GetAttribute gets an attribute of an element.
func (b *Browser) GetAttribute(ctx context.Context, selector, attr string) (string, error) {
	var value string
	err := chromedp.Run(b.ctx,
		chromedp.AttributeValue(selector, attr, &value, nil, chromedp.ByQuery),
	)
	return value, err
}

// Tabs returns the open tabs.
func (b *Browser) Tabs(ctx context.Context) ([]TabInfo, error) {
	// Get targets from CDP
	targets, err := chromedp.Targets(b.ctx)
	if err != nil {
		return nil, err
	}

	var tabs []TabInfo
	for _, t := range targets {
		if t.Type == "page" {
			tabs = append(tabs, TabInfo{
				ID:    string(t.TargetID),
				URL:   t.URL,
				Title: t.Title,
			})
		}
	}

	return tabs, nil
}

// TabInfo represents information about a browser tab.
type TabInfo struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Title string `json:"title"`
}
