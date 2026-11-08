// Command mercadona searches the Mercadona online store (tienda.mercadona.es) catalogue
// and prints the shared contract.Result shape, so the aggregator parses one format for
// every chain.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"multisuper/internal/contract"
	"multisuper/internal/mercadona"
)

const usage = `mercadona searches the Mercadona online store (tienda.mercadona.es) catalogue.

usage: mercadona search <term> [--limit N] [--wh CODE] [--json]

flags:
  --limit N  maximum number of hits, 1-100 (default 20)
  --wh C     warehouse whose catalogue and prices are read (default mad1). The
             assortment and the prices follow the warehouse that serves the
             shopper's postal code.
  --json     print the shared contract JSON envelope instead of a table

Mercadona's search index publishes no barcode and no stock state, so "ean" is absent
and "available" unknown. Every price is the one the store publishes for that warehouse.

exit status: 0 success, 1 fetch or parse error, 2 usage error.`

// searcher is the slice of mercadona.Client the command needs, so tests run offline.
type searcher interface {
	Search(ctx context.Context, warehouse, term string, limit int) (contract.Result, error)
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, mercadona.New()))
}

func run(args []string, stdout, stderr io.Writer, s searcher) int {
	// S3: asking for help is not an error, in any position. Left to the flag package
	// it returns flag.ErrHelp, which would be read as a usage error.
	for _, arg := range args {
		switch arg {
		case "-h", "--h", "-help", "--help":
			fmt.Fprintln(stdout, usage)
			return 0
		}
	}
	if len(args) > 0 {
		switch args[0] {
		case "help":
			fmt.Fprintln(stdout, usage)
			return 0
		case "search":
			return runSearch(args[1:], stdout, stderr, s)
		}
	}
	fmt.Fprintln(stderr, usage)
	return 2
}

func runSearch(args []string, stdout, stderr io.Writer, s searcher) int {
	fs := flag.NewFlagSet("mercadona search", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprintln(stderr, usage) }
	limit := fs.Int("limit", mercadona.DefaultLimit, "maximum number of hits (1-100)")
	warehouse := fs.String("wh", mercadona.DefaultWarehouse, "warehouse whose catalogue and prices are read")
	asJSON := fs.Bool("json", false, "print the contract.Result JSON instead of a table")

	// Parse in a loop so flags are accepted after the search term too: Go's flag
	// package stops at the first positional argument.
	var words []string
	for {
		if err := fs.Parse(args); err != nil {
			return 2
		}
		if fs.NArg() == 0 {
			break
		}
		words = append(words, fs.Arg(0))
		args = fs.Args()[1:]
	}
	term := strings.TrimSpace(strings.Join(words, " "))
	if term == "" {
		fs.Usage()
		return 2
	}
	// S3: a --limit outside 1..100 is a usage error, never silently clamped.
	if *limit < 1 || *limit > mercadona.MaxLimit {
		fmt.Fprintf(stderr, "--limit must be between 1 and %d, got %d\n", mercadona.MaxLimit, *limit)
		return 2
	}
	// S5: the warehouse is a path segment of the index, so it must be a slug; a
	// malformed one is rejected here instead of being sent to the store.
	if !mercadona.ValidWarehouse(*warehouse) {
		fmt.Fprintf(stderr, "mercadona: --wh must be a warehouse code such as %s, not %q\n", mercadona.DefaultWarehouse, *warehouse)
		return 2
	}

	res, err := s.Search(context.Background(), *warehouse, term, *limit)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	if *asJSON {
		if err := contract.Encode(stdout, res); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	if err := writeTable(stdout, res); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func writeTable(w io.Writer, res contract.Result) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tNAME\tPRICE\t€/UNIT")
	for _, h := range res.Hits {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", h.ID, h.Name, price(h), perUnit(h))
	}
	return tw.Flush()
}

// price prints "-" when the catalogue publishes none: 0.00 € would read as free.
func price(h contract.Hit) string {
	if h.Price == nil {
		return "-"
	}
	return fmt.Sprintf("%.2f €", *h.Price)
}

func perUnit(h contract.Hit) string {
	if h.PricePerUnit == 0 || h.Unit == "" {
		return "-"
	}
	return fmt.Sprintf("%.2f €/%s", h.PricePerUnit, h.Unit)
}
