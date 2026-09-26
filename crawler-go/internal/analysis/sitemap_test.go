package analysis

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSitemapParserFollowsIndex(t *testing.T) {
	const child = `<?xml version="1.0" encoding="UTF-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>https://example.com/a</loc></url></urlset>`
	var srv *httptest.Server
	index := func(decl, childPath string) string {
		return decl + `<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><sitemap><loc>` + srv.URL + childPath + `</loc></sitemap></sitemapindex>`
	}
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		utf8Decl := `<?xml version="1.0" encoding="UTF-8"?>`
		switch r.URL.Path {
		case "/plain.xml":
			w.Write([]byte(index(utf8Decl, "/child.xml")))
		case "/bom.xml":
			w.Write(append([]byte{0xEF, 0xBB, 0xBF}, index(utf8Decl, "/child.xml")...))
		case "/latin1.xml":
			w.Write([]byte(index(`<?xml version="1.0" encoding="ISO-8859-1"?>`, "/child.xml")))
		case "/gz.xml":
			w.Write([]byte(index(utf8Decl+"\n<?xml-stylesheet type=\"text/xsl\" href=\"/s.xsl\"?>\n", "/child.xml.gz")))
		case "/nested.xml":
			w.Write([]byte(index("", "/plain.xml")))
		case "/child.xml":
			w.Write([]byte(child))
		case "/child.xml.gz":
			var b bytes.Buffer
			zw := gzip.NewWriter(&b)
			zw.Write([]byte(child))
			zw.Close()
			w.Write(b.Bytes())
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	for _, name := range []string{"plain", "bom", "latin1", "gz", "nested"} {
		t.Run(name, func(t *testing.T) {
			res := NewSitemapParser().Parse([]string{srv.URL + "/" + name + ".xml"})
			if len(res.Errors) != 0 {
				t.Fatalf("errors: %v", res.Errors)
			}
			if len(res.URLs) != 1 || res.URLs[0] != "https://example.com/a" {
				t.Fatalf("urls = %v, want [https://example.com/a]", res.URLs)
			}
		})
	}
}
