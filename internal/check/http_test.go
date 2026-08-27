package check

import (
	"errors"
	"testing"
)

func TestCheckHTTP_AllOK(t *testing.T) {
	r := &Runner{
		Domain:      "example.com",
		ExpectedURL: "https://example.com/",
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
		Domain:      "example.com",
		ExpectedURL: "https://example.com/",
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

func TestCheckHTTP_HTTPSDowngrade(t *testing.T) {
	r := &Runner{
		Domain:      "example.com",
		ExpectedURL: "https://example.com/",
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
		Domain:      "example.com",
		ExpectedURL: "https://example.com/",
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
		Domain:      "example.com",
		ExpectedURL: "https://example.com/",
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
		Domain:      "example.com",
		ExpectedURL: "https://example.com/",
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
		Domain:      "example.com",
		ExpectedURL: "https://example.com/",
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
		Domain:      "example.com",
		ExpectedURL: "https://example.com/",
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
		Domain:      "sub.example.com",
		ExpectedURL: "https://sub.example.com/",
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

func fixedProbe(responses map[string]probeResult) func(string) probeResult {
	return func(url string) probeResult {
		if r, ok := responses[url]; ok {
			return r
		}
		return probeResult{Error: errors.New("unexpected URL")}
	}
}
