package mercadona

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"multisuper/internal/contract"
)

// fixture serves a captured Algolia response and records what the client sent, so the
// tests check the request the store actually receives, not an internal decision.
type fixture struct {
	*httptest.Server
	path    string
	body    string
	header  http.Header
	queries int
}

func serve(t *testing.T, name string) *fixture {
	t.Helper()
	body, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	f := &fixture{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.path, f.header = r.URL.Path, r.Header.Clone()
		raw, _ := io.ReadAll(r.Body)
		f.body = string(raw)
		f.queries++
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	t.Cleanup(f.Close)
	return f
}

func clientFor(f *fixture) *Client {
	return &Client{BaseURL: f.URL, HTTP: f.Client(), Sleep: func(context.Context, time.Duration) error { return nil }}
}

// params decodes the form-encoded Algolia "params" field the request carries.
func params(t *testing.T, body string) url.Values {
	t.Helper()
	var sent struct {
		Params string `json:"params"`
	}
	if err := json.Unmarshal([]byte(body), &sent); err != nil {
		t.Fatalf("request body is not {\"params\": \"...\"}: %v: %s", err, body)
	}
	v, err := url.ParseQuery(sent.Params)
	if err != nil {
		t.Fatalf("params are not a query string: %v: %s", err, sent.Params)
	}
	return v
}

// S5: the catalogue lives in one index per warehouse, and the request carries the public
// search-only credentials the store's own front end ships. Nothing is read from a config
// file or from the environment.
func TestSearchAsksTheWarehouseIndexWithThePublicKey(t *testing.T) {
	f := serve(t, "search_leche_entera.json")
	if _, err := clientFor(f).Search(context.Background(), "mad1", "leche entera", 3); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if want := "/1/indexes/products_prod_mad1_es/query"; f.path != want {
		t.Errorf("index path = %q, want %q", f.path, want)
	}
	if got := f.header.Get("X-Algolia-Application-Id"); got != searchAppID {
		t.Errorf("application id = %q, want %q", got, searchAppID)
	}
	if got := f.header.Get("X-Algolia-API-Key"); got != searchKey {
		t.Errorf("api key = %q, want the public search key", got)
	}
	q := params(t, f.body)
	if q.Get("query") != "leche entera" || q.Get("hitsPerPage") != "3" {
		t.Errorf("params = %v, want query=leche entera and hitsPerPage=3", q)
	}
}

// S4: every field comes from the published record. The price is unit_price; the unit
// price is reference_price with the unit of reference_format; the size is unit_size plus
// size_format. The EAN and the stock state are absent because the index publishes neither.
func TestSearchMapsThePublishedPriceFields(t *testing.T) {
	res, err := clientFor(serve(t, "search_leche_entera.json")).Search(context.Background(), "mad1", "leche entera", 3)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res.Store != "mercadona" || res.Query != "leche entera" || res.Count != 3 || len(res.Hits) != 3 {
		t.Fatalf("envelope = %+v", res)
	}

	pack := res.Hits[0]
	if pack.ID != "10379" || pack.Name != "Leche entera Hacendado" || pack.Brand != "Hacendado" {
		t.Errorf("identity fields = %+v", pack)
	}
	if pack.Price == nil || *pack.Price != 5.76 {
		t.Errorf("price = %v, want 5.76 (unit_price of the six-brick pack)", pack.Price)
	}
	if pack.Currency != "EUR" {
		t.Errorf("currency = %q, want EUR", pack.Currency)
	}
	if pack.PricePerUnit != 0.96 || pack.Unit != "L" {
		t.Errorf("unit price = %v %q, want 0.96 L (reference_price + reference_format)", pack.PricePerUnit, pack.Unit)
	}
	if pack.Size != "6 l" {
		t.Errorf("size = %q, want %q (unit_size + size_format)", pack.Size, "6 l")
	}
	if pack.Category != "Leche entera" {
		t.Errorf("category = %q, want the deepest published category", pack.Category)
	}
	if pack.URL != "https://tienda.mercadona.es/product/10379/leche-entera-hacendado-pack-6" {
		t.Errorf("url = %q, want the published share_url", pack.URL)
	}
	if !strings.HasPrefix(pack.Image, "https://prod-mercadona.imgix.net/") {
		t.Errorf("image = %q, want the published thumbnail", pack.Image)
	}
	if pack.Available != nil {
		t.Errorf("available = %v, want unknown: the index publishes no stock state", *pack.Available)
	}
	if pack.EAN != "" {
		t.Errorf("ean = %q, want empty: the search index publishes no barcode", pack.EAN)
	}
}

// S4: a product sold by weight keeps the same shape. The 200 g cheese at 1,60 € is
// 8,00 €/kg, and both numbers are the store's, not a division of our own.
func TestWeightedProductsKeepThePublishedUnitPrice(t *testing.T) {
	res, err := clientFor(serve(t, "search_queso_rallado.json")).Search(context.Background(), "mad1", "queso rallado", 2)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	cheese := res.Hits[0]
	if cheese.Price == nil || *cheese.Price != 1.60 {
		t.Fatalf("price = %v, want 1.60", cheese.Price)
	}
	if cheese.PricePerUnit != 8.00 || cheese.Unit != "kg" {
		t.Errorf("unit price = %v %q, want 8 kg", cheese.PricePerUnit, cheese.Unit)
	}
	if cheese.Size != "0.2 kg" {
		t.Errorf("size = %q, want %q", cheese.Size, "0.2 kg")
	}
	if cheese.Category != "Queso rallado" {
		t.Errorf("category = %q, want %q", cheese.Category, "Queso rallado")
	}
}

// S3: a limit outside 1..100 is a usage error, never a silent clamp, and it costs no
// request.
func TestLimitOutsideTheRangeIsRejected(t *testing.T) {
	for _, limit := range []int{0, -1, 101} {
		f := serve(t, "search_leche_entera.json")
		if _, err := clientFor(f).Search(context.Background(), "mad1", "leche", limit); err == nil {
			t.Errorf("limit %d was accepted", limit)
		}
		if f.queries != 0 {
			t.Errorf("limit %d sent %d requests, want none", limit, f.queries)
		}
	}
}

// S5: the warehouse is a path segment, so it must be a plain slug. Anything else is
// rejected before a request is built.
func TestWarehouseMustBeASlug(t *testing.T) {
	for _, wh := range []string{"", "  ", "../products_prod_mad1", "mad 1", "MAD1", "mad1/../../x"} {
		f := serve(t, "search_leche_entera.json")
		if _, err := clientFor(f).Search(context.Background(), wh, "leche", 3); err == nil {
			t.Errorf("warehouse %q was accepted", wh)
		}
		if f.queries != 0 {
			t.Errorf("warehouse %q sent a request, want none", wh)
		}
	}
}

// S6: a rejected key means the store rotated the credentials it publishes in its bundle.
// That is an explicit error, never an empty catalogue that reads as "nothing in stock".
func TestRejectedKeyIsNamedAsRotation(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			io.WriteString(w, `{"message":"Invalid Application-ID or API key"}`)
		}))
		c := &Client{BaseURL: srv.URL, HTTP: srv.Client(), Sleep: func(context.Context, time.Duration) error { return nil }}
		_, err := c.Search(context.Background(), "mad1", "leche", 3)
		srv.Close()
		if !errors.Is(err, ErrSearchKey) {
			t.Errorf("status %d: err = %v, want ErrSearchKey", status, err)
		}
	}
}

// S5: an unknown warehouse has no index. The error names it instead of blaming the network.
func TestUnknownWarehouseIsAnExplicitError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"message":"Index does not exist"}`)
	}))
	c := &Client{BaseURL: srv.URL, HTTP: srv.Client(), Sleep: func(context.Context, time.Duration) error { return nil }}
	_, err := c.Search(context.Background(), "zzz9", "leche", 3)
	srv.Close()
	if !errors.Is(err, ErrWarehouse) || !strings.Contains(err.Error(), "zzz9") {
		t.Errorf("err = %v, want ErrWarehouse naming the warehouse", err)
	}
}

// Throttling is retried, and the answer that finally arrives is the one returned.
func TestThrottlingIsRetried(t *testing.T) {
	body, err := os.ReadFile("testdata/search_leche_entera.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	var slept time.Duration
	c := &Client{BaseURL: srv.URL, HTTP: srv.Client(), Sleep: func(_ context.Context, d time.Duration) error {
		slept = d
		return nil
	}}
	res, err := c.Search(context.Background(), "mad1", "leche entera", 3)
	srv.Close()
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if calls != 2 || res.Count != 3 {
		t.Errorf("calls = %d, count = %d, want 2 and 3", calls, res.Count)
	}
	if slept != time.Second {
		t.Errorf("waited %v, want the 1s Retry-After the server sent", slept)
	}
}

// A reference format the contract cannot express leaves both fields empty: a unit price
// without a unit is a number nobody can compare (T21).
func TestUnknownReferenceFormatLeavesNoUnitPrice(t *testing.T) {
	raw := `{"hits":[{"id":"1","display_name":"Pan","price_instructions":{"unit_price":"1.20","reference_price":"0.30","reference_format":"100 g","unit_size":400,"size_format":"g"}}]}`
	res, err := searchAgainst(t, raw, 1)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	h := res.Hits[0]
	if h.PricePerUnit != 0 || h.Unit != "" {
		t.Errorf("unit price = %v %q, want empty for an unexpressible format", h.PricePerUnit, h.Unit)
	}
	if h.Size != "400 g" {
		t.Errorf("size = %q, want %q", h.Size, "400 g")
	}
}

// A product the index publishes without a price keeps an empty price, never 0 (S8).
func TestMissingPriceStaysEmpty(t *testing.T) {
	raw := `{"hits":[{"id":"2","display_name":"Plátano","price_instructions":{"unit_price":"","reference_price":"","reference_format":"","unit_size":0,"size_format":""}}]}`
	res, err := searchAgainst(t, raw, 1)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	h := res.Hits[0]
	if h.Price != nil {
		t.Errorf("price = %v, want nil", *h.Price)
	}
	if h.PricePerUnit != 0 || h.Unit != "" || h.Size != "" {
		t.Errorf("invented unit price or size: %+v", h)
	}
	if h.Currency != "" {
		t.Errorf("currency = %q, want empty without a price", h.Currency)
	}
}

// searchAgainst runs one search against a body the test wrote, for the shapes the live
// catalogue does not happen to produce today.
func searchAgainst(t *testing.T, body string, limit int) (contract.Result, error) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, body)
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, HTTP: srv.Client(), Sleep: func(context.Context, time.Duration) error { return nil }}
	return c.Search(context.Background(), "mad1", "pan", limit)
}
