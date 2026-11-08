// Package web holds the aggregator's front end as embedded files, so the server runs
// from any directory and the page is self-contained: no CDN, no web font, no external
// URL of any kind. The browser downloads four small files and nothing else.
package web

import "embed"

// Files is index.html, styles.css, app.js, the favicon and the brand mark, served by cmd/web.
// logo.png is the only raster: the horizontal multisuper wordmark, downscaled to the size the
// top bar renders so the page stays small.
//
//go:embed index.html styles.css app.js favicon.svg logo.png
var Files embed.FS
