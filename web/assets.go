package webassets

import "embed"

// Files embeds frontend assets so serverless runtimes can serve them reliably.
//
//go:embed index.html app.js config.js
var Files embed.FS

// IndexHTML is served for the root route without file-server redirects.
//
//go:embed index.html
var IndexHTML []byte
