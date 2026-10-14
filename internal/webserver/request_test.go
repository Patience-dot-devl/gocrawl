package webserver

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequireJSON(t *testing.T) {
	for _, tc := range []struct {
		contentType string
		want        bool
	}{
		{"application/json", true},
		{"application/json; charset=utf-8", true},
		{"text/plain", false},
		{"application/x-www-form-urlencoded", false},
		{"", false},
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/x", strings.NewReader(`{}`))
		if tc.contentType != "" {
			req.Header.Set("Content-Type", tc.contentType)
		}
		rec := httptest.NewRecorder()
		if got := requireJSON(rec, req); got != tc.want {
			t.Errorf("Content-Type %q: requireJSON = %v, want %v", tc.contentType, got, tc.want)
		}
		if !tc.want && rec.Code != http.StatusUnsupportedMediaType {
			t.Errorf("Content-Type %q: status = %d, want 415", tc.contentType, rec.Code)
		}
	}
}

// multipartBody builds a multipart/form-data body with one text field and one file part.
func multipartBody(t *testing.T, fileContent string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := mw.WriteField("domain", "example.com"); err != nil {
		t.Fatal(err)
	}
	fw, err := mw.CreateFormFile("file", "rules.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte(fileContent)); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, mw.FormDataContentType()
}

func TestParseMultipartAcceptsFormWithinLimit(t *testing.T) {
	body, ct := multipartBody(t, "source,target\n/a,/b\n")
	req := httptest.NewRequest(http.MethodPost, "/api/x", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	if !parseMultipart(rec, req, 1<<20) {
		t.Fatalf("parseMultipart = false, status %d, body %s", rec.Code, rec.Body.String())
	}
	defer req.MultipartForm.RemoveAll()
	if got := req.FormValue("domain"); got != "example.com" {
		t.Errorf("domain = %q, want example.com", got)
	}
	if _, _, err := req.FormFile("file"); err != nil {
		t.Errorf("FormFile(file): %v", err)
	}
}

func TestParseMultipartRejectsWrongContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/x", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	if parseMultipart(rec, req, 1<<20) {
		t.Fatal("parseMultipart = true for a JSON body")
	}
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415", rec.Code)
	}
}

func TestParseMultipartRejectsOversizedBody(t *testing.T) {
	body, ct := multipartBody(t, strings.Repeat("x", 4096))
	req := httptest.NewRequest(http.MethodPost, "/api/x", body)
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	if parseMultipart(rec, req, 1024) {
		t.Fatal("parseMultipart = true for a body over the limit")
	}
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413 (body %s)", rec.Code, rec.Body.String())
	}
}

func TestParseMultipartRejectsMalformedBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/x", strings.NewReader("not multipart"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xyz")
	rec := httptest.NewRecorder()
	if parseMultipart(rec, req, 1<<20) {
		t.Fatal("parseMultipart = true for a malformed body")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
