//go:build embed

package web

import (
	"compress/gzip"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type gzipResponseWriter struct {
	gin.ResponseWriter
	gzipWriter *gzip.Writer
}

func (w *gzipResponseWriter) WriteHeader(statusCode int) {
	if statusCode == http.StatusNoContent || statusCode == http.StatusNotModified || statusCode < http.StatusOK {
		w.Header().Del("Content-Encoding")
	} else {
		w.Header().Del("Content-Length")
	}
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *gzipResponseWriter) Write(data []byte) (int, error) {
	if !w.Written() {
		w.WriteHeader(http.StatusOK)
	}
	if w.Header().Get("Content-Encoding") != "gzip" {
		return w.ResponseWriter.Write(data)
	}
	if w.gzipWriter == nil {
		w.gzipWriter = gzip.NewWriter(w.ResponseWriter)
	}
	return w.gzipWriter.Write(data)
}

func (w *gzipResponseWriter) WriteString(data string) (int, error) {
	return w.Write([]byte(data))
}

func (w *gzipResponseWriter) Close() error {
	if w.gzipWriter == nil {
		return nil
	}
	return w.gzipWriter.Close()
}

func serveEmbeddedStaticFile(c *gin.Context, cleanPath string, fileServer http.Handler) {
	applyStaticAssetCacheHeaders(c.Writer.Header(), cleanPath)
	if !shouldCompressEmbeddedAsset(c.Request, cleanPath) {
		fileServer.ServeHTTP(c.Writer, c.Request)
		c.Abort()
		return
	}

	header := c.Writer.Header()
	header.Add("Vary", "Accept-Encoding")
	header.Set("Content-Encoding", "gzip")
	writer := &gzipResponseWriter{ResponseWriter: c.Writer}
	fileServer.ServeHTTP(writer, c.Request)
	if err := writer.Close(); err != nil {
		_ = c.Error(err)
	}
	c.Abort()
}

func shouldCompressEmbeddedAsset(request *http.Request, cleanPath string) bool {
	if request == nil || request.Method != http.MethodGet || request.Header.Get("Range") != "" {
		return false
	}
	switch strings.ToLower(filepath.Ext(cleanPath)) {
	case ".css", ".js", ".json", ".map", ".svg", ".txt", ".xml":
		return acceptsGzip(request.Header.Get("Accept-Encoding"))
	default:
		return false
	}
}

func acceptsGzip(value string) bool {
	gzipQuality := -1.0
	wildcardQuality := -1.0
	for _, item := range strings.Split(value, ",") {
		parts := strings.Split(item, ";")
		encoding := strings.ToLower(strings.TrimSpace(parts[0]))
		quality := 1.0
		for _, parameter := range parts[1:] {
			keyValue := strings.SplitN(strings.TrimSpace(parameter), "=", 2)
			if len(keyValue) != 2 || !strings.EqualFold(strings.TrimSpace(keyValue[0]), "q") {
				continue
			}
			parsed, err := strconv.ParseFloat(strings.TrimSpace(keyValue[1]), 64)
			if err != nil || parsed < 0 || parsed > 1 {
				quality = 0
			} else {
				quality = parsed
			}
		}
		switch encoding {
		case "gzip":
			gzipQuality = quality
		case "*":
			wildcardQuality = quality
		}
	}
	if gzipQuality >= 0 {
		return gzipQuality > 0
	}
	return wildcardQuality > 0
}
