package check

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestReadLimitedBody(t *testing.T) {
	_, err := readLimitedBody(strings.NewReader(strings.Repeat("x", maxContentBody+1)))
	if err == nil {
		t.Fatal("oversized response body was not rejected")
	}
}

func TestCheckHTTP_AllOK(t *testing.T) {
	r := &Runner{
		Domain:        "example.com",
		ExpectedHosts: []string{"example.com"},
	}

	probe := fixedProbe(map[string]probeResult{
		"http://example.com":      {StatusCode: 200, FinalURL: "https://example.com/"},
		"http://www.example.com":  {StatusCode: 200, FinalURL: "https://example.com/"},
		"https://www.example.com": {StatusCode: 200, FinalURL: "https://example.com/"},
		"https://example.com":     {StatusCode: 200, FinalURL: "https://example.com/"},
	})

	redirect, https, response, _ := r.checkHTTPWithProbe(probe)

	if redirect != OK {
		t.Errorf("redirect status = %v, want OK", redirect)
	}
	if https != OK {
		t.Errorf("https status = %v, want OK", https)
	}
	if response != OK {
		t.Errorf("response status = %v, want OK", response)
	}
	if r.failCount != 0 {
		t.Errorf("failCount = %d, want 0", r.failCount)
	}
}

func TestCheckHTTP_HTTPSRedirectToDifferentHost(t *testing.T) {
	// crawler.sh scenario: www redirects to www, but still over HTTPS.
	r := &Runner{
		Domain:        "example.com",
		ExpectedHosts: []string{"example.com"},
	}

	probe := fixedProbe(map[string]probeResult{
		"http://example.com":      {StatusCode: 200, FinalURL: "https://example.com/"},
		"http://www.example.com":  {StatusCode: 200, FinalURL: "https://www.example.com/"},
		"https://www.example.com": {StatusCode: 200, FinalURL: "https://www.example.com/"},
		"https://example.com":     {StatusCode: 200, FinalURL: "https://example.com/"},
	})

	redirect, https, _, _ := r.checkHTTPWithProbe(probe)

	if redirect != FAIL {
		t.Errorf("redirect status = %v, want FAIL", redirect)
	}
	if https != OK {
		t.Errorf("https status = %v, want OK", https)
	}
	if r.failCount != 2 {
		t.Errorf("failCount = %d, want 2", r.failCount)
	}
}

func TestCheckHTTP_MultipleExpectedHosts(t *testing.T) {
	// Both apex and www are acceptable canonical endpoints.
	r := &Runner{
		Domain:        "example.com",
		ExpectedHosts: []string{"example.com", "www.example.com"},
	}

	probe := fixedProbe(map[string]probeResult{
		"http://example.com":      {StatusCode: 200, FinalURL: "https://example.com/"},
		"http://www.example.com":  {StatusCode: 200, FinalURL: "https://www.example.com/"},
		"https://www.example.com": {StatusCode: 200, FinalURL: "https://www.example.com/"},
		"https://example.com":     {StatusCode: 200, FinalURL: "https://example.com/"},
	})

	redirect, https, _, _ := r.checkHTTPWithProbe(probe)

	if redirect != OK {
		t.Errorf("redirect status = %v, want OK", redirect)
	}
	if https != OK {
		t.Errorf("https status = %v, want OK", https)
	}
	if r.failCount != 0 {
		t.Errorf("failCount = %d, want 0", r.failCount)
	}
}

func TestCheckHTTP_HTTPSDowngrade(t *testing.T) {
	r := &Runner{
		Domain:        "example.com",
		ExpectedHosts: []string{"example.com"},
	}

	probe := fixedProbe(map[string]probeResult{
		"http://example.com":      {StatusCode: 200, FinalURL: "https://example.com/"},
		"http://www.example.com":  {StatusCode: 200, FinalURL: "https://example.com/"},
		"https://www.example.com": {StatusCode: 200, FinalURL: "https://example.com/"},
		"https://example.com":     {StatusCode: 200, FinalURL: "http://example.com/"},
	})

	redirect, https, _, _ := r.checkHTTPWithProbe(probe)

	if redirect != FAIL {
		t.Errorf("redirect status = %v, want FAIL", redirect)
	}
	if https != FAIL {
		t.Errorf("https status = %v, want FAIL", https)
	}
	if r.failCount != 1 {
		t.Errorf("failCount = %d, want 1", r.failCount)
	}
}

func TestCheckHTTP_HTTPSConnectionError(t *testing.T) {
	r := &Runner{
		Domain:        "example.com",
		ExpectedHosts: []string{"example.com"},
	}

	probe := fixedProbe(map[string]probeResult{
		"http://example.com":      {StatusCode: 200, FinalURL: "https://example.com/"},
		"http://www.example.com":  {StatusCode: 200, FinalURL: "https://example.com/"},
		"https://www.example.com": {StatusCode: 200, FinalURL: "https://example.com/"},
		"https://example.com":     {Error: errors.New("connection refused")},
	})

	redirect, https, _, _ := r.checkHTTPWithProbe(probe)

	if redirect != FAIL {
		t.Errorf("redirect status = %v, want FAIL", redirect)
	}
	if https != FAIL {
		t.Errorf("https status = %v, want FAIL", https)
	}
}

func TestCheckHTTP_HTTPToHTTPSExpectedHost(t *testing.T) {
	r := &Runner{
		Domain:        "example.com",
		ExpectedHosts: []string{"example.com"},
	}

	probe := fixedProbe(map[string]probeResult{
		"http://example.com":      {StatusCode: 200, FinalURL: "https://example.com/"},
		"http://www.example.com":  {StatusCode: 200, FinalURL: "https://example.com/"},
		"https://www.example.com": {StatusCode: 200, FinalURL: "https://example.com/"},
		"https://example.com":     {StatusCode: 200, FinalURL: "https://example.com/"},
	})

	redirect, https, _, _ := r.checkHTTPWithProbe(probe)

	if redirect != OK {
		t.Errorf("redirect status = %v, want OK", redirect)
	}
	if https != OK {
		t.Errorf("https status = %v, want OK", https)
	}
}

func TestCheckHTTP_Non200OnHTTPS(t *testing.T) {
	// HTTPS itself is fine (TLS handshake succeeded); the page just returned an error.
	r := &Runner{
		Domain:        "example.com",
		ExpectedHosts: []string{"example.com"},
	}

	probe := fixedProbe(map[string]probeResult{
		"http://example.com":      {StatusCode: 200, FinalURL: "https://example.com/"},
		"http://www.example.com":  {StatusCode: 200, FinalURL: "https://example.com/"},
		"https://www.example.com": {StatusCode: 200, FinalURL: "https://example.com/"},
		"https://example.com":     {StatusCode: 500, FinalURL: "https://example.com/"},
	})

	redirect, https, _, _ := r.checkHTTPWithProbe(probe)

	if redirect != FAIL {
		t.Errorf("redirect status = %v, want FAIL", redirect)
	}
	if https != OK {
		t.Errorf("https status = %v, want OK", https)
	}
}

func TestCheckHTTP_SlowResponseWarning(t *testing.T) {
	r := &Runner{
		Domain:        "example.com",
		ExpectedHosts: []string{"example.com"},
	}

	probe := fixedProbe(map[string]probeResult{
		"http://example.com":      {StatusCode: 200, FinalURL: "https://example.com/", TotalTime: 3500000000},
		"http://www.example.com":  {StatusCode: 200, FinalURL: "https://example.com/", TotalTime: 3500000000},
		"https://www.example.com": {StatusCode: 200, FinalURL: "https://example.com/", TotalTime: 3500000000},
		"https://example.com":     {StatusCode: 200, FinalURL: "https://example.com/", TotalTime: 3500000000},
	})

	redirect, https, response, _ := r.checkHTTPWithProbe(probe)

	if redirect != OK {
		t.Errorf("redirect status = %v, want OK", redirect)
	}
	if https != OK {
		t.Errorf("https status = %v, want OK", https)
	}
	if response != WARN {
		t.Errorf("response status = %v, want WARN", response)
	}
}

func TestCheckHTTP_VerySlowResponseFail(t *testing.T) {
	r := &Runner{
		Domain:        "example.com",
		ExpectedHosts: []string{"example.com"},
	}

	probe := fixedProbe(map[string]probeResult{
		"http://example.com":      {StatusCode: 200, FinalURL: "https://example.com/", TotalTime: 9000000000},
		"http://www.example.com":  {StatusCode: 200, FinalURL: "https://example.com/", TotalTime: 9000000000},
		"https://www.example.com": {StatusCode: 200, FinalURL: "https://example.com/", TotalTime: 9000000000},
		"https://example.com":     {StatusCode: 200, FinalURL: "https://example.com/", TotalTime: 9000000000},
	})

	redirect, https, response, _ := r.checkHTTPWithProbe(probe)

	if redirect != OK {
		t.Errorf("redirect status = %v, want OK", redirect)
	}
	if https != OK {
		t.Errorf("https status = %v, want OK", https)
	}
	if response != FAIL {
		t.Errorf("response status = %v, want FAIL", response)
	}
}

func TestCheckHTTP_Subdomain(t *testing.T) {
	r := &Runner{
		Domain:        "sub.example.com",
		ExpectedHosts: []string{"sub.example.com"},
	}

	probed := []string{}
	probe := func(url string) probeResult {
		probed = append(probed, url)
		return probeResult{StatusCode: 200, FinalURL: url}
	}

	_, _, _, _ = r.checkHTTPWithProbe(probe)

	want := []string{
		"http://sub.example.com",
		"https://sub.example.com",
	}
	if len(probed) != len(want) {
		t.Fatalf("probed %d URLs, want %d: %v", len(probed), len(want), probed)
	}
	for i, u := range want {
		if probed[i] != u {
			t.Errorf("probed[%d] = %q, want %q", i, probed[i], u)
		}
	}
}

func TestCheckHTTP_SkipRedirect(t *testing.T) {
	r := &Runner{
		Domain:        "example.com",
		ExpectedHosts: []string{"example.com"},
		SkipRedirect:  true,
	}

	probe := fixedProbe(map[string]probeResult{
		"http://example.com":      {StatusCode: 200, FinalURL: "https://example.com/"},
		"http://www.example.com":  {StatusCode: 200, FinalURL: "https://www.example.com/"},
		"https://www.example.com": {StatusCode: 200, FinalURL: "https://www.example.com/"},
		"https://example.com":     {StatusCode: 200, FinalURL: "https://example.com/"},
	})

	redirect, https, response, _ := r.checkHTTPWithProbe(probe)

	if redirect != SKIP {
		t.Errorf("redirect status = %v, want SKIP", redirect)
	}
	if https != OK {
		t.Errorf("https status = %v, want OK", https)
	}
	if response != OK {
		t.Errorf("response status = %v, want OK", response)
	}
	if r.failCount != 0 {
		t.Errorf("failCount = %d, want 0", r.failCount)
	}
}

func TestCheckHTTP_SkipRedirectIgnoresHTTPError(t *testing.T) {
	r := &Runner{
		Domain:        "example.com",
		ExpectedHosts: []string{"example.com"},
		SkipRedirect:  true,
	}

	probe := fixedProbe(map[string]probeResult{
		"http://example.com":      {Error: errors.New("connection refused")},
		"http://www.example.com":  {Error: errors.New("connection refused")},
		"https://www.example.com": {StatusCode: 200, FinalURL: "https://www.example.com/"},
		"https://example.com":     {StatusCode: 200, FinalURL: "https://example.com/"},
	})

	redirect, https, _, _ := r.checkHTTPWithProbe(probe)

	if redirect != SKIP {
		t.Errorf("redirect status = %v, want SKIP", redirect)
	}
	if https != OK {
		t.Errorf("https status = %v, want OK", https)
	}
	if r.failCount != 0 {
		t.Errorf("failCount = %d, want 0", r.failCount)
	}
}

func TestCheckHTTP_SkipRedirectStillFailsOnHTTPSDowngrade(t *testing.T) {
	r := &Runner{
		Domain:        "example.com",
		ExpectedHosts: []string{"example.com"},
		SkipRedirect:  true,
	}

	probe := fixedProbe(map[string]probeResult{
		"http://example.com":      {StatusCode: 200, FinalURL: "https://example.com/"},
		"http://www.example.com":  {StatusCode: 200, FinalURL: "https://example.com/"},
		"https://www.example.com": {StatusCode: 200, FinalURL: "https://example.com/"},
		"https://example.com":     {StatusCode: 200, FinalURL: "http://example.com/"},
	})

	redirect, https, _, _ := r.checkHTTPWithProbe(probe)

	if redirect != FAIL {
		t.Errorf("redirect status = %v, want FAIL", redirect)
	}
	if https != FAIL {
		t.Errorf("https status = %v, want FAIL", https)
	}
	if r.failCount != 1 {
		t.Errorf("failCount = %d, want 1", r.failCount)
	}
}

func TestCheckHTTP_SkipRedirectStillFailsOnHTTPSError(t *testing.T) {
	r := &Runner{
		Domain:        "example.com",
		ExpectedHosts: []string{"example.com"},
		SkipRedirect:  true,
	}

	probe := fixedProbe(map[string]probeResult{
		"http://example.com":      {StatusCode: 200, FinalURL: "https://example.com/"},
		"http://www.example.com":  {StatusCode: 200, FinalURL: "https://example.com/"},
		"https://www.example.com": {StatusCode: 200, FinalURL: "https://example.com/"},
		"https://example.com":     {Error: errors.New("tls: bad certificate")},
	})

	redirect, https, _, _ := r.checkHTTPWithProbe(probe)

	if redirect != SKIP {
		t.Errorf("redirect status = %v, want SKIP", redirect)
	}
	if https != FAIL {
		t.Errorf("https status = %v, want FAIL", https)
	}
	if r.failCount != 1 {
		t.Errorf("failCount = %d, want 1", r.failCount)
	}
}

func TestCheckLLMs_TextPlain(t *testing.T) {
	r := &Runner{Domain: "example.com"}

	client := &http.Client{
		Transport: &mockLLMsTransport{
			statusCode:  200,
			contentType: "text/plain; charset=utf-8",
			body:        []byte("# llms.txt"),
		},
	}

	r.checkLLMsWithClient(client)

	if r.warnCount != 0 {
		t.Errorf("warnCount = %d, want 0", r.warnCount)
	}
	if r.failCount != 0 {
		t.Errorf("failCount = %d, want 0", r.failCount)
	}
}

func TestCheckLLMs_Soft404HTML(t *testing.T) {
	// Many sites return their regular HTML page with 200 for /llms.txt.
	// This should be reported as a soft 404, not a pass.
	r := &Runner{Domain: "example.com"}

	client := &http.Client{
		Transport: &mockLLMsTransport{
			statusCode:  200,
			contentType: "text/html; charset=UTF-8",
			body:        []byte("<html><body>regular page</body></html>"),
		},
	}

	r.checkLLMsWithClient(client)

	if r.warnCount != 1 {
		t.Errorf("warnCount = %d, want 1", r.warnCount)
	}
}

func TestCheckLLMs_NotFound(t *testing.T) {
	r := &Runner{Domain: "example.com"}

	client := &http.Client{
		Transport: &mockLLMsTransport{
			statusCode:  404,
			contentType: "text/plain",
			body:        []byte("not found"),
		},
	}

	r.checkLLMsWithClient(client)

	if r.warnCount != 1 {
		t.Errorf("warnCount = %d, want 1", r.warnCount)
	}
}

func TestCheckLLMs_RequestError(t *testing.T) {
	r := &Runner{Domain: "example.com"}

	client := &http.Client{
		Transport: &mockLLMsTransport{
			err: errors.New("connection refused"),
		},
	}

	r.checkLLMsWithClient(client)

	if r.warnCount != 1 {
		t.Errorf("warnCount = %d, want 1", r.warnCount)
	}
}

type mockLLMsTransport struct {
	statusCode  int
	contentType string
	body        []byte
	err         error
}

func (t *mockLLMsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.err != nil {
		return nil, t.err
	}
	return &http.Response{
		StatusCode: t.statusCode,
		Header:     http.Header{"Content-Type": []string{t.contentType}},
		Body:       io.NopCloser(bytes.NewReader(t.body)),
		Request:    req,
	}, nil
}

func fixedProbe(responses map[string]probeResult) func(string) probeResult {
	return func(url string) probeResult {
		if r, ok := responses[url]; ok {
			return r
		}
		return probeResult{Error: errors.New("unexpected URL")}
	}
}
