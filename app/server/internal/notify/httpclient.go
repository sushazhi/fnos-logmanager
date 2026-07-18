package notify

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sushazhi/fnos-logmanager/internal/utils"
)

const defaultHTTPTimeout = 15 * time.Second

// httpClient is the package-level HTTP client with SSRF protection.
var sharedClient = &http.Client{
	Timeout: defaultHTTPTimeout,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return fmt.Errorf("too many redirects")
		}
		// Block redirects to private IPs (SSRF protection)
		if utils.IsPrivateURL(req.URL.String()) {
			return fmt.Errorf("redirect blocked: target is a private address")
		}
		return nil
	},
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			// Only allow TLS 1.2+
			MinVersion: tls.VersionTLS12,
		},
		Dial: (&net.Dialer{
			Timeout: 10 * time.Second,
		}).Dial,
	},
}

// HttpRequestOptions holds optional parameters for an HTTP request.
type HttpRequestOptions struct {
	Method  string
	Headers map[string]string
	JSON    interface{}
	Form    map[string]string
	Body    string
	Timeout time.Duration
}

// HttpResponse represents an HTTP response.
type HttpResponse struct {
	StatusCode int
	Body       string
	Headers    map[string]string
}

// httpRequest sends an HTTP request with SSRF protection.
func httpRequest(rawURL string, opts HttpRequestOptions) (*HttpResponse, error) {
	// Validate URL and reject private addresses
	if utils.IsPrivateURL(rawURL) {
		return nil, fmt.Errorf("request blocked: %s is a private address", rawURL)
	}

	method := opts.Method
	if method == "" {
		method = "GET"
	}

	var reqBody io.Reader
	headers := make(map[string]string)
	for k, v := range opts.Headers {
		headers[k] = v
	}

	if opts.JSON != nil {
		data, err := json.Marshal(opts.JSON)
		if err != nil {
			return nil, fmt.Errorf("json marshal: %w", err)
		}
		reqBody = bytes.NewReader(data)
		if headers["content-type"] == "" {
			headers["content-type"] = "application/json"
		}
	} else if opts.Form != nil {
		formData := url.Values{}
		for k, v := range opts.Form {
			formData.Set(k, v)
		}
		reqBody = strings.NewReader(formData.Encode())
		if headers["content-type"] == "" {
			headers["content-type"] = "application/x-www-form-urlencoded"
		}
	} else if opts.Body != "" {
		reqBody = strings.NewReader(opts.Body)
	}

	req, err := http.NewRequest(method, rawURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = defaultHTTPTimeout
	}
	client := sharedClient
	if timeout != defaultHTTPTimeout {
		client = &http.Client{
			Timeout:       timeout,
			CheckRedirect: sharedClient.CheckRedirect,
			Transport:     sharedClient.Transport,
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	respHeaders := make(map[string]string)
	for k := range resp.Header {
		respHeaders[k] = resp.Header.Get(k)
	}

	return &HttpResponse{
		StatusCode: resp.StatusCode,
		Body:       string(body),
		Headers:    respHeaders,
	}, nil
}

// HTTPRequest sends an HTTP request with the given options.
// Supports custom methods, headers, JSON/Form/Body payloads, and timeouts.
func HTTPRequest(rawURL string, opts HttpRequestOptions) (*HttpResponse, error) {
	return httpRequest(rawURL, opts)
}

// HTTPPost is a convenience method for POST requests.
func HTTPPost(rawURL string, opts HttpRequestOptions) (*HttpResponse, error) {
	opts.Method = "POST"
	return httpRequest(rawURL, opts)
}

// HTTPGet is a convenience method for GET requests.
func HTTPGet(rawURL string, opts HttpRequestOptions) (*HttpResponse, error) {
	opts.Method = "GET"
	return httpRequest(rawURL, opts)
}


