package s3x

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"s3scalpel/internal/model"
)

// fakeS3 is a deliberately small in-memory S3 implementation: enough of the REST
// surface for the listing, transfer and delete paths to be exercised end to end
// against a real *s3.Client (signing, pagination, ranged GETs and multipart
// assembly all included) without needing network access or credentials.
type fakeS3 struct {
	mu      sync.Mutex
	objects map[string][]byte
	uploads map[string]map[int32][]byte // uploadId -> partNumber -> bytes

	// Test hooks.
	failPut      map[string]int    // key -> remaining PutObject failures to inject
	failAllParts bool              // reject every UploadPart
	failPartNums map[int32]bool    // reject these part numbers (survives SDK retries)
	partPuts     int               // successful UploadPart calls served
	rangeReqs    int               // number of ranged GETs served
	listPageMax  int               // objects per ListObjectsV2 page (0 = unlimited)
	deleted      []string          // keys deleted, in service order
	nextUpload   int               // number of multipart uploads created
	uploadKeys   map[string]string // uploadId -> object key
}

func newFakeS3() *fakeS3 {
	return &fakeS3{
		objects:      map[string][]byte{},
		uploads:      map[string]map[int32][]byte{},
		uploadKeys:   map[string]string{},
		failPut:      map[string]int{},
		failPartNums: map[int32]bool{},
	}
}

// start serves the fake over httptest and returns a path-style client for it.
func (f *fakeS3) start(t *testing.T) *s3.Client {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)

	cl, err := build(context.Background(), model.Connection{
		ID:        "fake",
		Endpoint:  srv.URL,
		Region:    "us-east-1",
		PathStyle: true,
		AccessKey: "key",
		SecretKey: "secret",
	})
	if err != nil {
		t.Fatalf("build client: %v", err)
	}
	return cl
}

func (f *fakeS3) put(key string, data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[key] = data
}

func (f *fakeS3) get(key string) ([]byte, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.objects[key]
	return b, ok
}

func (f *fakeS3) keys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.objects))
	for k := range f.objects {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Path-style: /<bucket>/<key...>
	trimmed := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.SplitN(trimmed, "/", 2)
	key := ""
	if len(parts) == 2 {
		key = parts[1]
	}
	q := r.URL.Query()

	switch {
	case r.Method == http.MethodGet && q.Has("uploads"):
		f.listMultipartUploads(w)
	case r.Method == http.MethodGet && q.Has("uploadId"):
		f.listParts(w, q)
	case r.Method == http.MethodGet && q.Get("list-type") == "2":
		f.listObjects(w, q)
	case r.Method == http.MethodPost && q.Has("uploads"):
		f.createMultipart(w, key)
	case r.Method == http.MethodPut && q.Has("uploadId"):
		f.uploadPart(w, r, q)
	case r.Method == http.MethodPost && q.Has("uploadId"):
		f.completeMultipart(w, r, key, q)
	case r.Method == http.MethodDelete && q.Has("uploadId"):
		f.mu.Lock()
		id := q.Get("uploadId")
		delete(f.uploads, id)
		delete(f.uploadKeys, id)
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodPut:
		f.putObject(w, r, key)
	case r.Method == http.MethodHead:
		f.headObject(w, key)
	case r.Method == http.MethodGet:
		f.getObject(w, r, key)
	case r.Method == http.MethodDelete:
		f.deleteObject(w, key)
	default:
		w.WriteHeader(http.StatusNotImplemented)
	}
}

func (f *fakeS3) listObjects(w http.ResponseWriter, q map[string][]string) {
	get := func(k string) string {
		if v := q[k]; len(v) > 0 {
			return v[0]
		}
		return ""
	}
	prefix, delim, token := get("prefix"), get("delimiter"), get("continuation-token")

	all := f.keys()
	var matched []string
	for _, k := range all {
		if strings.HasPrefix(k, prefix) {
			matched = append(matched, k)
		}
	}

	start := 0
	if token != "" {
		start, _ = strconv.Atoi(token)
	}
	end := len(matched)
	truncated := false
	if f.listPageMax > 0 && start+f.listPageMax < end {
		end = start + f.listPageMax
		truncated = true
	}
	page := matched[start:end]

	type object struct {
		Key          string `xml:"Key"`
		Size         int64  `xml:"Size"`
		ETag         string `xml:"ETag"`
		StorageClass string `xml:"StorageClass"`
	}
	type commonPrefix struct {
		Prefix string `xml:"Prefix"`
	}
	type result struct {
		XMLName               xml.Name       `xml:"ListBucketResult"`
		IsTruncated           bool           `xml:"IsTruncated"`
		NextContinuationToken string         `xml:"NextContinuationToken,omitempty"`
		Contents              []object       `xml:"Contents"`
		CommonPrefixes        []commonPrefix `xml:"CommonPrefixes"`
	}

	res := result{IsTruncated: truncated}
	if truncated {
		res.NextContinuationToken = strconv.Itoa(end)
	}
	seenPrefix := map[string]bool{}
	for _, k := range page {
		rest := strings.TrimPrefix(k, prefix)
		if delim != "" {
			if idx := strings.Index(rest, delim); idx >= 0 {
				cp := prefix + rest[:idx+len(delim)]
				if !seenPrefix[cp] {
					seenPrefix[cp] = true
					res.CommonPrefixes = append(res.CommonPrefixes, commonPrefix{Prefix: cp})
				}
				continue
			}
		}
		data, _ := f.get(k)
		res.Contents = append(res.Contents, object{
			Key: k, Size: int64(len(data)), ETag: `"etag"`, StorageClass: "STANDARD",
		})
	}
	w.Header().Set("Content-Type", "application/xml")
	_ = xml.NewEncoder(w).Encode(res)
}

func (f *fakeS3) putObject(w http.ResponseWriter, r *http.Request, key string) {
	f.mu.Lock()
	if n := f.failPut[key]; n > 0 {
		f.failPut[key] = n - 1
		f.mu.Unlock()
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	f.mu.Unlock()

	body, _ := io.ReadAll(r.Body)
	f.put(key, body)
	w.Header().Set("ETag", `"etag"`)
	w.WriteHeader(http.StatusOK)
}

func (f *fakeS3) headObject(w http.ResponseWriter, key string) {
	data, ok := f.get(key)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("ETag", `"etag"`)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
}

func (f *fakeS3) getObject(w http.ResponseWriter, r *http.Request, key string) {
	data, ok := f.get(key)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if rng := r.Header.Get("Range"); rng != "" {
		var lo, hi int64
		if _, err := fmt.Sscanf(rng, "bytes=%d-%d", &lo, &hi); err == nil {
			if hi >= int64(len(data)) {
				hi = int64(len(data)) - 1
			}
			f.mu.Lock()
			f.rangeReqs++
			f.mu.Unlock()
			slice := data[lo : hi+1]
			w.Header().Set("Content-Length", strconv.Itoa(len(slice)))
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", lo, hi, len(data)))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(slice)
			return
		}
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = w.Write(data)
}

func (f *fakeS3) deleteObject(w http.ResponseWriter, key string) {
	f.mu.Lock()
	delete(f.objects, key)
	f.deleted = append(f.deleted, key)
	f.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (f *fakeS3) createMultipart(w http.ResponseWriter, key string) {
	f.mu.Lock()
	f.nextUpload++
	id := fmt.Sprintf("upload-%d", f.nextUpload)
	f.uploads[id] = map[int32][]byte{}
	f.uploadKeys[id] = key
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/xml")
	_, _ = fmt.Fprintf(w, `<InitiateMultipartUploadResult><Bucket>b</Bucket><Key>%s</Key><UploadId>%s</UploadId></InitiateMultipartUploadResult>`, key, id)
}

func (f *fakeS3) uploadPart(w http.ResponseWriter, r *http.Request, q map[string][]string) {
	id := q["uploadId"][0]
	num, _ := strconv.Atoi(q["partNumber"][0])

	f.mu.Lock()
	reject := f.failAllParts || f.failPartNums[int32(num)]
	f.mu.Unlock()
	if reject {
		// A 500 is retriable, so the SDK will re-attempt it; the rejection is
		// keyed on the part number rather than a countdown so the outcome is
		// deterministic regardless of how many retries the SDK makes.
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	body, _ := io.ReadAll(r.Body)

	f.mu.Lock()
	if f.uploads[id] == nil {
		f.uploads[id] = map[int32][]byte{}
	}
	f.uploads[id][int32(num)] = body
	f.partPuts++
	f.mu.Unlock()

	w.Header().Set("ETag", fmt.Sprintf(`"part-%d"`, num))
	w.WriteHeader(http.StatusOK)
}

func (f *fakeS3) completeMultipart(w http.ResponseWriter, r *http.Request, key string, q map[string][]string) {
	id := q["uploadId"][0]

	var payload struct {
		Parts []struct {
			PartNumber int32 `xml:"PartNumber"`
		} `xml:"Part"`
	}
	body, _ := io.ReadAll(r.Body)
	_ = xml.Unmarshal(body, &payload)

	f.mu.Lock()
	parts := f.uploads[id]
	var assembled []byte
	ordered := true
	last := int32(0)
	for _, p := range payload.Parts {
		if p.PartNumber <= last {
			ordered = false
		}
		last = p.PartNumber
		assembled = append(assembled, parts[p.PartNumber]...)
	}
	delete(f.uploads, id)
	delete(f.uploadKeys, id)
	f.objects[key] = assembled
	f.mu.Unlock()

	if !ordered {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`<Error><Code>InvalidPartOrder</Code></Error>`))
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	_, _ = fmt.Fprintf(w, `<CompleteMultipartUploadResult><Bucket>b</Bucket><Key>%s</Key><ETag>"final"</ETag></CompleteMultipartUploadResult>`, key)
}

// listMultipartUploads answers GET /<bucket>?uploads.
func (f *fakeS3) listMultipartUploads(w http.ResponseWriter) {
	type upload struct {
		Key      string `xml:"Key"`
		UploadID string `xml:"UploadId"`
	}
	type result struct {
		XMLName     xml.Name `xml:"ListMultipartUploadsResult"`
		IsTruncated bool     `xml:"IsTruncated"`
		Uploads     []upload `xml:"Upload"`
	}
	f.mu.Lock()
	ids := make([]string, 0, len(f.uploadKeys))
	for id := range f.uploadKeys {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	res := result{}
	for _, id := range ids {
		res.Uploads = append(res.Uploads, upload{Key: f.uploadKeys[id], UploadID: id})
	}
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/xml")
	_ = xml.NewEncoder(w).Encode(res)
}

// listParts answers GET /<bucket>/<key>?uploadId=...
func (f *fakeS3) listParts(w http.ResponseWriter, q map[string][]string) {
	type part struct {
		PartNumber int32  `xml:"PartNumber"`
		Size       int64  `xml:"Size"`
		ETag       string `xml:"ETag"`
	}
	type result struct {
		XMLName     xml.Name `xml:"ListPartsResult"`
		IsTruncated bool     `xml:"IsTruncated"`
		Parts       []part   `xml:"Part"`
	}
	id := q["uploadId"][0]

	f.mu.Lock()
	stored, ok := f.uploads[id]
	var numbers []int
	for n := range stored {
		numbers = append(numbers, int(n))
	}
	sort.Ints(numbers)
	res := result{}
	for _, n := range numbers {
		res.Parts = append(res.Parts, part{
			PartNumber: int32(n),
			Size:       int64(len(stored[int32(n)])),
			ETag:       fmt.Sprintf(`"part-%d"`, n),
		})
	}
	f.mu.Unlock()

	if !ok {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<Error><Code>NoSuchUpload</Code></Error>`))
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	_ = xml.NewEncoder(w).Encode(res)
}
