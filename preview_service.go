package main

import (
	"context"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"s3scalpel/internal/model"
	"s3scalpel/internal/s3x"
)

// previewURLExpiry bounds the life of the presigned URL used to stream audio and
// video previews.
const previewURLExpiry = time.Hour

// PreviewService prepares object previews. Images, PDFs and text are downloaded
// to a temp directory (bounded by the preview size limit) and returned as data;
// audio/video are streamed from a presigned URL.
type PreviewService struct{ core *Core }

// GetPreview returns preview data for an object.
//
// Images and PDFs are downloaded to the cache directory and handed back as a
// local URL rather than a base64 data: URL — inlining costs roughly 2.3x the
// object size in live memory and blocks until the whole thing is encoded. Text
// is read with a single ranged GET capped at the preview limit, so a huge log
// file costs one bounded read and no temp file. Audio and video stream from a
// presigned URL.
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
	limit := s.core.Settings().PreviewMaxSize

	switch kind {
	case model.PreviewMedia:
		// Long enough to watch a film, short enough that a URL leaked from the
		// webview (devtools, a copied link) stops working the same day.
		url, err := s3x.PresignGet(ctx, cl, bucket, key, "", previewURLExpiry)
		if err != nil {
			return out, err
		}
		out.URL = url
		return out, nil

	case model.PreviewUnsupported:
		return out, nil

	case model.PreviewText:
		// Reading only the first `limit` bytes means an oversized text file is
		// still previewable — truncated — instead of refused outright.
		data, truncated, err := s3x.ReadRange(ctx, cl, bucket, key, limit)
		if err != nil {
			return out, err
		}
		out.Text = string(data)
		out.Truncated = truncated
		return out, nil
	}

	if props.Size > limit {
		out.Kind = model.PreviewTooLarge
		return out, nil
	}

	// Stage the object on disk and let the webview stream it from the app's own
	// asset server. The file is deleted as soon as it has been fetched once.
	tmpDir := filepath.Join(s.core.cacheDir, "preview")
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		return out, err
	}
	token := randID()
	tmp := filepath.Join(tmpDir, token+filepath.Ext(key))
	if err := s3x.Download(ctx, cl, bucket, key, tmp, s3x.DownloadOptions{}, nil); err != nil {
		return out, err
	}
	out.URL = s.core.previews.stage(token, tmp, ct)
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
