package s3x

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"s3scalpel/internal/model"
)

// TestIntegration exercises the S3 layer against a real S3-compatible endpoint.
// It is skipped unless the S3SCALPEL_TEST_* environment variables are set, so it
// never embeds credentials and is safe to keep in the repository.
//
//	S3SCALPEL_TEST_ENDPOINT, S3SCALPEL_TEST_ACCESS, S3SCALPEL_TEST_SECRET
//	S3SCALPEL_TEST_REGION (optional), S3SCALPEL_TEST_PATHSTYLE ("1" for path-style)
func TestIntegration(t *testing.T) {
	endpoint := os.Getenv("S3SCALPEL_TEST_ENDPOINT")
	access := os.Getenv("S3SCALPEL_TEST_ACCESS")
	secret := os.Getenv("S3SCALPEL_TEST_SECRET")
	if endpoint == "" || access == "" || secret == "" {
		t.Skip("set S3SCALPEL_TEST_ENDPOINT/ACCESS/SECRET to run the integration test")
	}

	conn := model.Connection{
		ID:        "test",
		Endpoint:  endpoint,
		Region:    os.Getenv("S3SCALPEL_TEST_REGION"),
		AccessKey: access,
		SecretKey: secret,
		PathStyle: os.Getenv("S3SCALPEL_TEST_PATHSTYLE") == "1",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	mgr := NewManager()
	cl, err := mgr.Client(ctx, conn)
	if err != nil {
		t.Fatalf("client: %v", err)
	}

	// ListBuckets may be denied on accounts whose keys are scoped to specific
	// buckets; that is not fatal to object-level testing.
	buckets, err := ListBuckets(ctx, cl)
	if err != nil {
		t.Logf("ListBuckets denied/failed (continuing): %v", err)
	} else {
		t.Logf("found %d buckets", len(buckets))
	}

	bucket := os.Getenv("S3SCALPEL_TEST_BUCKET")
	switch {
	case bucket != "":
		// use the provided bucket
	case len(buckets) > 0:
		bucket = buckets[0].Name
	default:
		bucket = fmt.Sprintf("s3scalpel-it-%d", time.Now().Unix())
		if err := CreateBucket(ctx, cl, bucket, conn.Region); err != nil {
			t.Fatalf("no bucket available and CreateBucket %s failed: %v (set S3SCALPEL_TEST_BUCKET to an existing bucket)", bucket, err)
		}
		t.Logf("created bucket %s", bucket)
	}
	t.Logf("using bucket %s", bucket)

	prefix := fmt.Sprintf("__s3scalpel_it__/%d/", time.Now().UnixNano())
	key := prefix + "hello.txt"
	content := []byte("hello from s3 scalpel integration test\n")

	// Write a local temp file and upload it.
	tmp := filepath.Join(t.TempDir(), "hello.txt")
	if err := os.WriteFile(tmp, content, 0o600); err != nil {
		t.Fatal(err)
	}
	var lastProgress int64
	if err := Upload(ctx, cl, bucket, key, tmp, false, 0, UploadOptions{}, func(n int64) { lastProgress = n }); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if lastProgress != int64(len(content)) {
		t.Errorf("progress=%d want %d", lastProgress, len(content))
	}

	// Folder-style listing should see the object under the prefix.
	res, err := ListObjects(ctx, cl, bucket, prefix, "", 1000)
	if err != nil {
		t.Fatalf("ListObjects: %v", err)
	}
	found := false
	for _, e := range res.Entries {
		if e.Key == key {
			found = true
			if e.Size != int64(len(content)) {
				t.Errorf("listed size=%d want %d", e.Size, len(content))
			}
		}
	}
	if !found {
		t.Errorf("uploaded key %s not found in listing", key)
	}

	// HeadObject.
	props, err := HeadObject(ctx, cl, bucket, key, "")
	if err != nil {
		t.Fatalf("HeadObject: %v", err)
	}
	if props.Size != int64(len(content)) {
		t.Errorf("head size=%d want %d", props.Size, len(content))
	}

	// Download and verify content.
	dst := filepath.Join(t.TempDir(), "hello-dl.txt")
	if err := Download(ctx, cl, bucket, key, dst, DownloadOptions{}, nil); err != nil {
		t.Fatalf("Download: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("download mismatch: got %q want %q", got, content)
	}

	// Tagging round-trip.
	if err := PutTags(ctx, cl, bucket, key, []model.Tag{{Key: "env", Value: "test"}}); err != nil {
		t.Fatalf("PutTags: %v", err)
	}
	tags, err := GetTags(ctx, cl, bucket, key)
	if err != nil {
		t.Fatalf("GetTags: %v", err)
	}
	if len(tags) != 1 || tags[0].Key != "env" || tags[0].Value != "test" {
		t.Errorf("tags=%v", tags)
	}

	// Presign a GET and fetch it over HTTP.
	url, err := PresignGet(ctx, cl, bucket, key, "", 5*time.Minute)
	if err != nil {
		t.Fatalf("PresignGet: %v", err)
	}
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET presigned: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || !bytes.Equal(body, content) {
		t.Errorf("presigned GET status=%d body=%q", resp.StatusCode, body)
	}

	// Multipart upload path (force multipart with a >5MiB object).
	big := make([]byte, 6*1024*1024)
	for i := range big {
		big[i] = byte(i % 251)
	}
	bigLocal := filepath.Join(t.TempDir(), "big.bin")
	if err := os.WriteFile(bigLocal, big, 0o600); err != nil {
		t.Fatal(err)
	}
	bigKey := prefix + "big.bin"
	if err := Upload(ctx, cl, bucket, bigKey, bigLocal, true, MinPartSize, UploadOptions{}, nil); err != nil {
		t.Fatalf("multipart Upload: %v", err)
	}
	bigDst := filepath.Join(t.TempDir(), "big-dl.bin")
	if err := Download(ctx, cl, bucket, bigKey, bigDst, DownloadOptions{Multipart: true, PartSize: MinPartSize, PartConcurrency: 3}, nil); err != nil {
		t.Fatalf("multipart Download: %v", err)
	}
	gotBig, _ := os.ReadFile(bigDst)
	if !bytes.Equal(gotBig, big) {
		t.Errorf("multipart download mismatch (len got=%d want=%d)", len(gotBig), len(big))
	}

	// Capability probe (logged, non-fatal) against the live bucket.
	for _, c := range CheckCapabilities(ctx, cl, bucket) {
		t.Logf("capability %-18s tested=%v allowed=%v %s", c.Op, c.Tested, c.Allowed, c.Detail)
	}

	// Clean up the test objects (leave any pre-existing bucket intact).
	if err := DeletePrefix(ctx, cl, bucket, prefix); err != nil {
		t.Errorf("cleanup DeletePrefix: %v", err)
	}
	t.Log("integration test passed")
}
