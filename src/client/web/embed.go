// Package web holds the embedded browser client (xterm.js) served by the
// server. It is a client implementation; the server serializes the wire format
// and this package only ships the static assets that render it.
package web

import "embed"

// Assets is the embedded web client (index.html and any future static files).
//
//go:embed index.html
var Assets embed.FS
