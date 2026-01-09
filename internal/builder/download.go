package builder

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

// DefaultDialTimeout is the default timeout for establishing connections.
const DefaultDialTimeout = 30 * time.Second

// DefaultResponseHeaderTimeout is the default timeout for receiving response headers.
const DefaultResponseHeaderTimeout = 30 * time.Second

// Downloader handles downloading files with resume support.
type Downloader struct {
	client *http.Client
}

// DownloaderOption configures a Downloader.
type DownloaderOption func(*Downloader)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) DownloaderOption {
	return func(d *Downloader) {
		d.client = client
	}
}

// WithTimeout sets the timeout for HTTP operations.
func WithTimeout(timeout time.Duration) DownloaderOption {
	return func(d *Downloader) {
		d.client = &http.Client{
			Timeout: timeout,
		}
	}
}

// NewDownloader creates a new Downloader with sensible defaults.
func NewDownloader(opts ...DownloaderOption) *Downloader {
	d := &Downloader{
		client: &http.Client{
			Timeout: 0, // No overall timeout - we handle it per-request.
			Transport: &http.Transport{
				ResponseHeaderTimeout: DefaultResponseHeaderTimeout,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		},
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Download downloads a URL to a file with resume support.
// Returns the total size and a reader for the content.
func (d *Downloader) Download(ctx context.Context, url string, destPath string) (io.ReadCloser, int64, error) {
	// Check if partial file exists.
	var existingSize int64
	if info, err := os.Stat(destPath); err == nil {
		existingSize = info.Size()
	}

	// Create request with range header for resume.
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("creating request: %w", err)
	}

	if existingSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingSize))
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("downloading: %w", err)
	}

	// Check response status.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		resp.Body.Close()
		return nil, 0, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	// Get total size.
	var totalSize int64
	if resp.StatusCode == http.StatusPartialContent {
		// Parse Content-Range header.
		contentRange := resp.Header.Get("Content-Range")
		if contentRange != "" {
			// Format: bytes 0-999/1234
			var start, end int64
			_, err := fmt.Sscanf(contentRange, "bytes %d-%d/%d", &start, &end, &totalSize)
			if err != nil {
				totalSize = existingSize + resp.ContentLength
			}
		}
	} else {
		totalSize = resp.ContentLength
		existingSize = 0 // Server didn't support range, start over.
	}

	return resp.Body, totalSize, nil
}

// DownloadToFile downloads a URL directly to a file.
func (d *Downloader) DownloadToFile(ctx context.Context, url string, destPath string, progress ProgressFunc) error {
	body, totalSize, err := d.Download(ctx, url, destPath)
	if err != nil {
		return err
	}
	defer body.Close()

	// Check existing size for append mode.
	var existingSize int64
	var flags int
	if info, err := os.Stat(destPath); err == nil {
		existingSize = info.Size()
		flags = os.O_WRONLY | os.O_APPEND
	} else {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}

	file, err := os.OpenFile(destPath, flags, 0644)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	// Copy with progress.
	buf := make([]byte, 32*1024)
	var downloaded int64 = existingSize

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, err := body.Read(buf)
		if n > 0 {
			if _, writeErr := file.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("writing file: %w", writeErr)
			}
			downloaded += int64(n)

			if progress != nil {
				progress(Progress{
					Phase:           "download",
					BytesDownloaded: downloaded,
					BytesTotal:      totalSize,
				})
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading response: %w", err)
		}
	}

	return nil
}

// GetContentLength gets the content length of a URL without downloading.
func (d *Downloader) GetContentLength(ctx context.Context, url string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return 0, err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	lengthStr := resp.Header.Get("Content-Length")
	if lengthStr == "" {
		return 0, nil
	}

	return strconv.ParseInt(lengthStr, 10, 64)
}
