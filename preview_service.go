package main

import (
	"context"
	"encoding/base64"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"s3scalpel/internal/model"
	"s3scalpel/internal/s3x"
)

// PreviewService prepares object previews. Images, PDFs and text are downloaded
// to a temp directory (bounded by the preview size limit) and returned as data;
// audio/video are streamed from a presigned URL.
type PreviewService struct{ core *Core }

// GetPreview returns preview data for an object.
func (s *PreviewService) GetPreview(connID, bucket, key string) (model.PreviewData, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cl, _, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return model.PreviewData{}, err
	}

	props, err := s3x.HeadObject(ctx, cl, bucket, key, "")
	if err != nil {
		return model.PreviewData{}, err
	}
	ct := props.ContentType
	if ct == "" || ct == "application/octet-stream" {
		if guess := mime.TypeByExtension(filepath.Ext(key)); guess != "" {
			ct = guess
		}
	}
	kind := classify(ct, key)
	out := model.PreviewData{Kind: kind, ContentType: ct, Size: props.Size}

	switch kind {
	case model.PreviewMedia:
		url, err := s3x.PresignGet(ctx, cl, bucket, key, "", 6*time.Hour)
		if err != nil {
			return out, err
		}
		out.URL = url
		return out, nil
	case model.PreviewUnsupported:
		return out, nil
	}

	limit := s.core.Settings().PreviewMaxSize
	if props.Size > limit {
		out.Kind = model.PreviewTooLarge
		return out, nil
	}

	// Download to a temp file (honouring "download to temp dir before preview").
	tmpDir := filepath.Join(s.core.cacheDir, "preview")
	_ = os.MkdirAll(tmpDir, 0o755)
	tmp := filepath.Join(tmpDir, randID()+filepath.Ext(key))
	if err := s3x.Download(ctx, cl, bucket, key, tmp, nil); err != nil {
		return out, err
	}
	defer os.Remove(tmp)

	data, err := os.ReadFile(tmp)
	if err != nil {
		return out, err
	}
	switch kind {
	case model.PreviewText:
		out.Text = string(data)
	case model.PreviewImage, model.PreviewPDF:
		out.DataURL = "data:" + ct + ";base64," + base64.StdEncoding.EncodeToString(data)
	}
	return out, nil
}

func classify(ct, key string) model.PreviewKind {
	lct := strings.ToLower(ct)
	switch {
	case strings.HasPrefix(lct, "image/"):
		return model.PreviewImage
	case lct == "application/pdf":
		return model.PreviewPDF
	case strings.HasPrefix(lct, "audio/"), strings.HasPrefix(lct, "video/"):
		return model.PreviewMedia
	case strings.HasPrefix(lct, "text/"), isTextual(lct), isTextExt(key):
		return model.PreviewText
	default:
		return model.PreviewUnsupported
	}
}

func isTextual(ct string) bool {
	for _, t := range []string{"application/json", "application/xml", "application/javascript", "application/x-yaml", "application/x-sh", "application/toml"} {
		if strings.HasPrefix(ct, t) {
			return true
		}
	}
	return false
}

func isTextExt(key string) bool {
	ext := strings.ToLower(filepath.Ext(key))
	switch ext {
	case ".txt", ".md", ".json", ".xml", ".yaml", ".yml", ".toml", ".ini", ".conf",
		".csv", ".log", ".js", ".ts", ".jsx", ".tsx", ".go", ".py", ".rb", ".rs",
		".java", ".c", ".cpp", ".h", ".hpp", ".cs", ".php", ".sh", ".bash", ".sql",
		".html", ".css", ".scss", ".vue", ".svelte", ".env", ".gitignore", ".dockerfile":
		return true
	}
	return false
}
