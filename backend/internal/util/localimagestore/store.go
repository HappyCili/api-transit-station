package localimagestore

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultDataDir = "./data"
	URLPathPrefix  = "/storage/api-images"
)

type StoredImage struct {
	FileName    string
	Path        string
	ContentType string
	Bytes       int
}

func StorageDir(dataDir string) string {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		dataDir = DefaultDataDir
	}
	return filepath.Join(dataDir, "api-images")
}

func Store(dataDir string, data []byte, contentType string) (StoredImage, error) {
	if len(data) == 0 {
		return StoredImage{}, fmt.Errorf("image data is empty")
	}
	contentType = normalizeImageContentType(contentType, data)
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		return StoredImage{}, fmt.Errorf("file is not an image")
	}
	dir := StorageDir(dataDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return StoredImage{}, fmt.Errorf("create image storage dir: %w", err)
	}

	sum := sha256.Sum256(data)
	fileName := hex.EncodeToString(sum[:]) + extensionForContentType(contentType)
	path := filepath.Join(dir, fileName)
	if _, err := os.Stat(path); err == nil {
		return StoredImage{FileName: fileName, Path: path, ContentType: contentType, Bytes: len(data)}, nil
	} else if !os.IsNotExist(err) {
		return StoredImage{}, fmt.Errorf("stat stored image: %w", err)
	}

	tmp, err := os.CreateTemp(dir, "."+fileName+".tmp-*")
	if err != nil {
		return StoredImage{}, fmt.Errorf("create temp image file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return StoredImage{}, fmt.Errorf("write image file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return StoredImage{}, fmt.Errorf("close image file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		if _, statErr := os.Stat(path); statErr == nil {
			return StoredImage{FileName: fileName, Path: path, ContentType: contentType, Bytes: len(data)}, nil
		}
		return StoredImage{}, fmt.Errorf("store image file: %w", err)
	}
	return StoredImage{FileName: fileName, Path: path, ContentType: contentType, Bytes: len(data)}, nil
}

func DecodeDataURL(value string) ([]byte, string, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(strings.ToLower(value), "data:") {
		return nil, "", fmt.Errorf("not a data url")
	}
	header, encoded, ok := strings.Cut(value, ",")
	if !ok {
		return nil, "", fmt.Errorf("invalid data url")
	}
	mediaType := ""
	if len(header) >= len("data:") {
		mediaType = header[len("data:"):]
	}
	if semicolon := strings.Index(mediaType, ";"); semicolon >= 0 {
		mediaType = mediaType[:semicolon]
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, "", err
	}
	return data, mediaType, nil
}

func PublicURL(r *http.Request, frontendURL string, fileName string) string {
	escapedName := url.PathEscape(strings.TrimSpace(fileName))
	path := URLPathPrefix + "/" + escapedName
	if strings.TrimSpace(escapedName) == "" {
		return URLPathPrefix
	}
	base := strings.TrimRight(strings.TrimSpace(frontendURL), "/")
	if base != "" {
		return base + path
	}
	if r == nil {
		return path
	}
	host := firstHeaderValue(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return path
	}
	scheme := strings.ToLower(firstHeaderValue(r.Header.Get("X-Forwarded-Proto")))
	if scheme != "http" && scheme != "https" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	return scheme + "://" + host + path
}

func SafeFileName(fileName string) (string, bool) {
	fileName = strings.TrimSpace(fileName)
	if fileName == "" || strings.Contains(fileName, "/") || strings.Contains(fileName, "\\") {
		return "", false
	}
	clean := filepath.Base(fileName)
	if clean != fileName || strings.HasPrefix(clean, ".") {
		return "", false
	}
	return clean, true
}

func normalizeImageContentType(contentType string, data []byte) string {
	contentType = strings.TrimSpace(contentType)
	if mediaType, _, err := mime.ParseMediaType(contentType); err == nil {
		contentType = mediaType
	}
	if contentType == "" || strings.EqualFold(contentType, "application/octet-stream") {
		contentType = http.DetectContentType(data)
	}
	return strings.ToLower(strings.TrimSpace(contentType))
}

func extensionForContentType(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".img"
	}
}

func firstHeaderValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if idx := strings.Index(value, ","); idx >= 0 {
		value = value[:idx]
	}
	return strings.TrimSpace(value)
}
