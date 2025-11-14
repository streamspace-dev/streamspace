package middleware

import (
	"compress/gzip"
	"io"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// Gzip compression levels
const (
	DefaultCompression = gzip.DefaultCompression
	NoCompression      = gzip.NoCompression
	BestSpeed          = gzip.BestSpeed
	BestCompression    = gzip.BestCompression
)

// Pool of gzip writers for reuse
var gzipWriterPool = sync.Pool{
	New: func() interface{} {
		return gzip.NewWriter(io.Discard)
	},
}

// gzipWriter wraps gin.ResponseWriter with gzip compression
type gzipWriter struct {
	gin.ResponseWriter
	writer *gzip.Writer
}

func (g *gzipWriter) Write(data []byte) (int, error) {
	return g.writer.Write(data)
}

func (g *gzipWriter) WriteString(s string) (int, error) {
	return g.writer.Write([]byte(s))
}

// Gzip returns a middleware that compresses HTTP responses using gzip
func Gzip(level int) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip compression for:
		// 1. WebSocket requests
		// 2. Server-Sent Events
		// 3. Clients that don't support gzip
		if !shouldCompress(c.Request) {
			c.Next()
			return
		}

		// Get a gzip writer from the pool
		gz := gzipWriterPool.Get().(*gzip.Writer)
		defer gzipWriterPool.Put(gz)

		// Reset the writer for this response
		gz.Reset(c.Writer)
		defer gz.Close()

		// Set compression level
		if level != DefaultCompression {
			gz.Close() // Close the default writer
			var err error
			gz, err = gzip.NewWriterLevel(c.Writer, level)
			if err != nil {
				c.Next()
				return
			}
			defer gz.Close()
		}

		// Set response headers
		c.Header("Content-Encoding", "gzip")
		c.Header("Vary", "Accept-Encoding")

		// Wrap the response writer
		c.Writer = &gzipWriter{
			ResponseWriter: c.Writer,
			writer:         gz,
		}

		// Process the request
		c.Next()

		// Ensure all data is written
		gz.Flush()
	}
}

// shouldCompress determines if the response should be compressed
func shouldCompress(r *gin.Context.Request) bool {
	// Check if client accepts gzip
	if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		return false
	}

	// Skip WebSocket connections
	if r.Header.Get("Upgrade") == "websocket" {
		return false
	}

	// Skip Server-Sent Events
	if r.Header.Get("Accept") == "text/event-stream" {
		return false
	}

	return true
}

// GzipWithExclusions returns a middleware with path exclusions
func GzipWithExclusions(level int, excludePaths []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if path should be excluded
		for _, path := range excludePaths {
			if strings.HasPrefix(c.Request.URL.Path, path) {
				c.Next()
				return
			}
		}

		// Use regular gzip middleware
		Gzip(level)(c)
	}
}
