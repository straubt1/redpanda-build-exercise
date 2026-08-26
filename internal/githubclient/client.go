package githubclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// File budget from decisions.md (working default).
const (
	MaxFiles      = 20
	MaxPatchChars = 4000
)

type Client struct {
	token     string
	userAgent string
	http      *http.Client
}

func New(token, userAgent string) *Client {
	if userAgent == "" {
		userAgent = "redpanda-build-exercise"
	}
	return &Client{
		token:     token,
		userAgent: userAgent,
		http:      &http.Client{Timeout: 30 * time.Second},
	}
}

type HTTPError struct {
	Status int
	URL    string
	Body   string
}

func (e *HTTPError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("github HTTP %d %s", e.Status, e.URL)
	}
	return fmt.Sprintf("github HTTP %d %s: %s", e.Status, e.URL, e.Body)
}

func StatusOf(err error) int {
	var he *HTTPError
	if errors.As(err, &he) {
		return he.Status
	}
	return 0
}

type File struct {
	Filename string `json:"filename"`
	Status   string `json:"status"`
	Patch    string `json:"patch"`
}

// Enrichment is the pull + files payload for later skip-LLM / model steps.
type Enrichment struct {
	Title   string
	Body    string
	HTMLURL string
	Author  string
	Files   []File
}

type pullJSON struct {
	Title   string `json:"title"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"`
	User    struct {
		Login string `json:"login"`
	} `json:"user"`
}

func (c *Client) Fetch(ctx context.Context, repo string, prNumber int) (*Enrichment, error) {
	owner, name, err := splitRepo(repo)
	if err != nil {
		return nil, err
	}

	pr, err := c.getPull(ctx, owner, name, prNumber)
	if err != nil {
		return nil, err
	}
	files, err := c.getFiles(ctx, owner, name, prNumber)
	if err != nil {
		return nil, err
	}

	return &Enrichment{
		Title:   pr.Title,
		Body:    pr.Body,
		HTMLURL: pr.HTMLURL,
		Author:  pr.User.Login,
		Files:   applyBudget(files),
	}, nil
}

func (c *Client) getPull(ctx context.Context, owner, repo string, n int) (*pullJSON, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", owner, repo, n)
	b, err := c.get(ctx, url)
	if err != nil {
		return nil, err
	}
	var pr pullJSON
	if err := json.Unmarshal(b, &pr); err != nil {
		return nil, fmt.Errorf("decode pull: %w", err)
	}
	return &pr, nil
}

func (c *Client) getFiles(ctx context.Context, owner, repo string, n int) ([]File, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d/files?per_page=%d", owner, repo, n, MaxFiles)
	b, err := c.get(ctx, url)
	if err != nil {
		return nil, err
	}
	var files []File
	if err := json.Unmarshal(b, &files); err != nil {
		return nil, fmt.Errorf("decode files: %w", err)
	}
	return files, nil
}

func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := strings.TrimSpace(string(body))
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return nil, &HTTPError{Status: resp.StatusCode, URL: url, Body: snippet}
	}
	return body, nil
}

func applyBudget(files []File) []File {
	if len(files) > MaxFiles {
		files = files[:MaxFiles]
	}
	for i := range files {
		if len(files[i].Patch) > MaxPatchChars {
			files[i].Patch = files[i].Patch[:MaxPatchChars]
		}
	}
	return files
}

func splitRepo(repo string) (owner, name string, err error) {
	owner, name, ok := strings.Cut(strings.TrimSpace(repo), "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return "", "", fmt.Errorf("invalid repo %q (want owner/name)", repo)
	}
	return owner, name, nil
}
