// Package web is the web-client host for the tmux-free server: it embeds the
// browser client (xterm.js UI) and provides Handler, which serves that UI under
// a strict Content-Security-Policy and reverse-proxies the data plane (/api,
// /ws) to the headless backend (cmd/server). The browser talks only to this
// origin; the backend serves no HTML. Wired up by cmd/web.
package web

import "embed"

// Assets is the embedded web client: the page, its script, and the vendored
// xterm.js bundle. Everything is served same-origin so a strict
// Content-Security-Policy (script-src 'self') applies — nothing is loaded from
// a CDN, eliminating the third-party-script supply-chain risk.
//
//go:embed index.html app.js vendor
var Assets embed.FS
