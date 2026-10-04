package serve

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The /donate page must carry copy buttons wired to the clipboard so
// users can paste the addresses instead of retyping 42 chars.
func TestDonatePageHasCopyButtons(t *testing.T) {
	_, mux := newTestServe(t)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/donate")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	for _, want := range []string{
		`data-copy="` + DonateEthereum + `"`,
		`data-copy="` + DonateBitcoin + `"`,
		"copyAddress", // the JS handler
	} {
		if !strings.Contains(html, want) {
			t.Errorf("donate page missing %q:\n%s", want, html)
		}
	}
}
