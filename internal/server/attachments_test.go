package server_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
)

// upload posts one multipart file part (field "file").
func (c *client) upload(path, field, name, ctype, content string, edit ...func(*http.Request)) (int, map[string]any) {
	c.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("note", "ignored")
	h := map[string][]string{"Content-Disposition": {`form-data; name="` + field + `"; filename="` + name + `"`}, "Content-Type": {ctype}}
	part, _ := mw.CreatePart(h)
	_, _ = io.WriteString(part, content)
	_ = mw.Close()
	req, _ := http.NewRequest("POST", c.srv.URL+path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if c.host != "" {
		req.Host = c.host
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	for _, e := range edit {
		e(req)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	spec.check(c.t, req, res)
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

func (c *client) fetch(path string) (*http.Response, string) {
	c.t.Helper()
	res := c.raw("GET", path, nil, nil)
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res, string(b)
}

// Attachments (C5): files on issues follow the issue's permissions and download safely.
func TestAttachments(t *testing.T) {
	forEachBackend(t, func(t *testing.T, anon *client) {
		lead := anon.signedIn("lead@x.uz")
		vic := anon.signedIn("vic@x.uz")
		out := anon.signedIn("out@x.uz")
		lead.do("POST", "/api/v1/projects", map[string]string{"key": "KYB", "name": "Kyber"})
		lead.do("POST", "/api/v1/projects/KYB/members", map[string]string{"email": "vic@x.uz", "role": "viewer"})
		lead.do("POST", "/api/v1/projects/KYB/issues", map[string]string{"title": "Crash", "type": "bug"})
		const url = "/api/v1/issues/KYB-1/attachments"

		code, a := lead.upload(url, "file", "../trace.html", "text/html", "<script>alert(1)</script>")
		expect(t, code, 201, a)
		if a["name"] != "trace.html" || a["size"] != float64(25) || a["inline"] != false || a["issue_key"] != "KYB-1" {
			t.Fatalf("uploaded = %v", a)
		}
		code, img := lead.upload(url, "file", "shot.png", "image/png", "\x89PNG")
		expect(t, code, 201, img)
		code, b := vic.upload(url, "file", "x.txt", "text/plain", "x")
		expect(t, code, 403, b)
		code, b = out.upload(url, "file", "x.txt", "text/plain", "x")
		expect(t, code, 404, b)
		code, b = lead.upload(url, "other", "x.txt", "text/plain", "x")
		expect(t, code, 422, b)
		code, b = lead.upload(url, "file", "empty.txt", "text/plain", "")
		expect(t, code, 422, b)
		code, b = lead.do("POST", url, map[string]string{"not": "multipart"})
		expect(t, code, 422, b)
		code, b = lead.upload(url, "file", "big.bin", "application/octet-stream", strings.Repeat("x", 25<<20+1))
		expect(t, code, 413, b)

		// The web app uploads with its session cookie: a custom header proves the request is same-origin.
		viaCookie := func(r *http.Request) {
			r.Header.Del("Authorization")
			r.AddCookie(&http.Cookie{Name: "kyber_session", Value: lead.token})
		}
		code, b = lead.upload(url, "file", "csrf.txt", "text/plain", "x", viaCookie)
		expect(t, code, 403, b)
		code, b = lead.upload(url, "file", "notes.txt", "text/plain", "ok", viaCookie, func(r *http.Request) { r.Header.Set("X-Kyber-Request", "1") })
		expect(t, code, 201, b)
		lead.do("DELETE", "/api/v1/attachments/"+b["id"].(string), nil)

		code, list := vic.do("GET", url, nil)
		expect(t, code, 200, list)
		if items := list["items"].([]any); len(items) != 2 {
			t.Fatalf("list = %v", list)
		}
		code, b = out.do("GET", url, nil)
		expect(t, code, 404, b)

		res, body := vic.fetch(a["url"].(string))
		if res.StatusCode != 200 || body != "<script>alert(1)</script>" ||
			res.Header.Get("Content-Disposition") != `attachment; filename=trace.html` ||
			res.Header.Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(res.Header.Get("Content-Security-Policy"), "sandbox") {
			t.Fatalf("download: %d %q %v", res.StatusCode, body, res.Header)
		}
		res, _ = vic.fetch(img["url"].(string))
		if res.Header.Get("Content-Disposition") != `inline; filename=shot.png` || res.Header.Get("Content-Type") != "image/png" {
			t.Fatalf("image headers: %v", res.Header)
		}
		res, _ = vic.fetch(img["url"].(string) + "?download")
		if !strings.HasPrefix(res.Header.Get("Content-Disposition"), "attachment") {
			t.Fatalf("forced download: %v", res.Header)
		}
		if res, _ := out.fetch(a["url"].(string)); res.StatusCode != 404 {
			t.Fatalf("outsider download: %d", res.StatusCode)
		}

		aurl := "/api/v1/attachments/" + a["id"].(string)
		code, b = vic.do("DELETE", aurl, nil)
		expect(t, code, 403, b)
		code, b = lead.do("DELETE", aurl, nil)
		expect(t, code, 204, b)
		code, b = lead.do("DELETE", aurl, nil)
		expect(t, code, 404, b)
		if res, _ := lead.fetch(aurl); res.StatusCode != 404 {
			t.Fatalf("deleted download: %d", res.StatusCode)
		}
	})
}
