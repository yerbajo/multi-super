// Package mercadona searches the Mercadona online store (tienda.mercadona.es) and
// normalises its products into contract.Hit, so the aggregator compares it with the six
// chains it already has.
//
// Discovery (2026-11): the store's front end searches a public **Algolia** index, one per
// warehouse and language: products_prod_<warehouse>_es. The application id and the
// search-only key travel inside the JavaScript bundle any visitor downloads, so they are
// public credentials, not leaked secrets — the same situation as ALDI's search key. The
// anonymous read needs no token, no cookie and no browser fingerprint: a plain HTTP client
// gets 200, which is why this package has no dependency beyond the standard library.
//
// Prices are **per warehouse** and so is the catalogue: `wh` is a path segment of the
// index, `mad1` is the Madrid warehouse and the default here, and a warehouse the store
// does not serve answers 404 instead of a national price we would have invented.
//
// What the index does not publish, this package does not fill: there is no barcode (only
// GET /api/products/<id>/ has one) and no stock state (`published` means published in the
// catalogue, not in stock), so `ean` stays absent and `available` stays unknown.
package mercadona

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"multisuper/internal/contract"
)

const (
	// Store is the chain's key in the shared contract, the capability table and the web.
	Store = "mercadona"
	// DefaultLimit is how many hits a search returns unless --limit says otherwise.
	DefaultLimit = 20
	// MaxLimit caps --limit, matching the index page size the store's own search uses.
	MaxLimit = 100
	// DefaultWarehouse is the warehouse whose catalogue and prices are read when no --wh
	// is given: the store's own default for a visitor with no postal code set.
	DefaultWarehouse = "mad1"

	// searchAppID and searchKey are Mercadona's **public search-only** Algolia
	// credentials, the pair tienda.mercadona.es ships in its front-end bundle, which any
	// visitor downloads. Measured: a query answers 200; the pair cannot write (it is a
	// search key). They rotate when the bundle is rebuilt, and then the search fails with
	// ErrSearchKey rather than returning an empty catalogue.
	searchAppID = "7UZJKL1DJ0"
	searchKey   = "9d8f2e39e90df472b4f2e559a116fe17"
	// searchHost serves the Algolia search API for searchAppID.
	searchHost = "https://7uzjkl1dj0-dsn.algolia.net"

	// indexFormat is the per-warehouse index: the catalogue and the prices follow the
	// warehouse that serves the shopper's postal code.
	indexFormat = "/1/indexes/products_prod_%s_es/query"

	userAgent    = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
	httpTimeout  = 20 * time.Second
	maxAttempts  = 3
	maxRetryWait = 10 * time.Second
)

var (
	// ErrUnavailable reports that Algolia kept answering 429/503 after all retries.
	ErrUnavailable = errors.New("mercadona search unavailable after retries")
	// ErrSearchKey reports a rejected public search key, which means the store rotated
	// the credentials embedded in its front-end bundle.
	ErrSearchKey = errors.New("mercadona: the public Algolia search key in the tienda.mercadona.es bundle was rejected; it has probably rotated")
	// ErrWarehouse reports an index that does not exist: the store does not serve that
	// warehouse, so there is no catalogue and no price to report for it.
	ErrWarehouse = errors.New("mercadona: no catalogue index for that warehouse")
)

// reWarehouse keeps the warehouse a plain slug: it is a path segment of the index, so a
// value carrying a slash or a space must never reach the URL.
var reWarehouse = regexp.MustCompile(`^[a-z][a-z0-9]{1,11}$`)

// ValidWarehouse reports whether wh names a warehouse-shaped code. It cannot know which
// warehouses exist today — the store publishes no list — so an unknown but well-formed
// code fails with ErrWarehouse when the index answers 404.
func ValidWarehouse(wh string) bool { return reWarehouse.MatchString(wh) }

// Client talks to the Algolia index behind the tienda.mercadona.es search box.
type Client struct {
	// BaseURL is the Algolia host; tests point it at a fake server.
	BaseURL string
	HTTP    *http.Client
	// Sleep waits between retries; tests replace it to keep Retry-After honoured
	// without slowing the suite.
	Sleep func(context.Context, time.Duration) error
}

// New returns a client for the store's public search index.
func New() *Client {
	return &Client{
		BaseURL: searchHost,
		HTTP:    &http.Client{Timeout: httpTimeout},
		Sleep:   sleepContext,
	}
}

// query is the Algolia request body: the site's own front end sends the search as a
// form-encoded "params" string, not as flat JSON fields.
type query struct {
	Params string `json:"params"`
}

// hit is the part of an Algolia product record this package reads. The record is much
// fatter (badges, highlight results, previous prices); nothing absent here is derived.
type hit struct {
	ID         string     `json:"id"`
	Name       string     `json:"display_name"`
	Brand      string     `json:"brand"`
	ShareURL   string     `json:"share_url"`
	Thumbnail  string     `json:"thumbnail"`
	Categories []category `json:"categories"`
	// Price holds the published numbers. They are strings in the store's own payload
	// ("5.76"), so they are parsed, never reformatted from a float we invented.
	Price priceInstructions `json:"price_instructions"`
}

// category is one node of the published tree; the deepest name is the useful one.
type category struct {
	Name       string     `json:"name"`
	Categories []category `json:"categories"`
}

// priceInstructions is the priced block shared by search hits and product detail.
// unit_price is what the package costs; reference_price is the €/unit the shelf shows and
// reference_format names that unit; unit_size and size_format are the sold size.
type priceInstructions struct {
	UnitPrice       string  `json:"unit_price"`
	ReferencePrice  string  `json:"reference_price"`
	ReferenceFormat string  `json:"reference_format"`
	UnitSize        float64 `json:"unit_size"`
	SizeFormat      string  `json:"size_format"`
}

// Search runs one full-text query against the warehouse index and returns it in the
// shared envelope. A limit outside 1..MaxLimit is rejected, never clamped, so the CLI can
// report it as a usage error (S3).
func (c *Client) Search(ctx context.Context, warehouse, term string, limit int) (contract.Result, error) {
	res := contract.Result{Store: Store, Query: term, Hits: []contract.Hit{}}
	if limit < 1 || limit > MaxLimit {
		return res, fmt.Errorf("mercadona: limit must be between 1 and %d, got %d", MaxLimit, limit)
	}
	if !ValidWarehouse(warehouse) {
		return res, fmt.Errorf("%w: %q is not a warehouse code", ErrWarehouse, warehouse)
	}

	params := url.Values{}
	params.Set("query", term)
	params.Set("hitsPerPage", strconv.Itoa(limit))
	body, err := json.Marshal(query{Params: params.Encode()})
	if err != nil {
		return res, fmt.Errorf("mercadona: encoding query: %w", err)
	}

	endpoint := strings.TrimSuffix(c.BaseURL, "/") + fmt.Sprintf(indexFormat, warehouse)
	raw, err := c.post(ctx, endpoint, body)
	if err != nil {
		if errors.Is(err, ErrWarehouse) {
			return res, fmt.Errorf("%w: %q", ErrWarehouse, warehouse)
		}
		return res, err
	}

	var payload struct {
		Hits []hit `json:"hits"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return res, fmt.Errorf("mercadona: decode search response: %w", err)
	}
	for _, h := range payload.Hits {
		res.Hits = append(res.Hits, h.contractHit())
	}
	res.Count = len(res.Hits)
	return res, nil
}

// contractHit normalises one catalogue record. An empty price stays empty (S4, S8).
func (h hit) contractHit() contract.Hit {
	out := contract.Hit{
		Store:    Store,
		ID:       h.ID,
		Name:     h.Name,
		Brand:    h.Brand,
		Size:     h.Price.size(),
		Category: deepestCategory(h.Categories),
		URL:      h.ShareURL,
		Image:    h.Thumbnail,
	}
	if out.Price = h.Price.price(); out.Price != nil {
		out.Currency = "EUR"
	}
	out.PricePerUnit, out.Unit = h.Price.unitPrice()
	return out
}

// deepestCategory returns the last published node, which is the one that names the
// product ("Leche entera"), not the top of the tree ("Huevos, leche y mantequilla").
func deepestCategory(cats []category) string {
	name := ""
	for _, c := range cats {
		name = c.Name
		if deeper := deepestCategory(c.Categories); deeper != "" {
			name = deeper
		}
	}
	return name
}

// post sends the search request, retrying 429/503 up to maxAttempts in total.
func (c *Client) post(ctx context.Context, endpoint string, body []byte) ([]byte, error) {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("mercadona: build request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Referer", "https://tienda.mercadona.es/")
		req.Header.Set("X-Algolia-Application-Id", searchAppID)
		req.Header.Set("X-Algolia-API-Key", searchKey)

		resp, err := c.HTTP.Do(req)
		if err != nil {
			return nil, fmt.Errorf("mercadona: search request: %w", err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusOK:
			if readErr != nil {
				return nil, fmt.Errorf("mercadona: read response: %w", readErr)
			}
			return raw, nil
		case http.StatusUnauthorized, http.StatusForbidden:
			return nil, fmt.Errorf("%w (status %s)", ErrSearchKey, resp.Status)
		case http.StatusNotFound:
			return nil, ErrWarehouse
		case http.StatusTooManyRequests, http.StatusServiceUnavailable:
			lastErr = fmt.Errorf("status %s", resp.Status)
			if attempt == maxAttempts {
				break
			}
			if err := c.Sleep(ctx, retryAfter(resp, attempt)); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("mercadona: search status %s", resp.Status)
		}
	}
	return nil, fmt.Errorf("%w (last %v)", ErrUnavailable, lastErr)
}

func sleepContext(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// retryAfter honours the Retry-After header in seconds, else backs off exponentially.
func retryAfter(resp *http.Response, attempt int) time.Duration {
	if secs, err := strconv.Atoi(strings.TrimSpace(resp.Header.Get("Retry-After"))); err == nil && secs >= 0 {
		return min(time.Duration(secs)*time.Second, maxRetryWait)
	}
	return min(time.Duration(attempt)*500*time.Millisecond, maxRetryWait)
}
