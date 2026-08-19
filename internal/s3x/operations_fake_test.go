package s3x

import (
	"bytes"
	"context"
	"crypto/rand"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"
	"testing"
)

// storeMax records the high-water mark of a concurrently reported progress
// value. ProgressFunc is invoked from several goroutines during parallel
// transfers, so plain assignment would itself be a race.
func storeMax(dst *atomic.Int64, v int64) {
	for {
		cur := dst.Load()
		if v <= cur || dst.CompareAndSwap(cur, v) {
			return
		}
	}
}

func TestListObjectsSplitsFoldersAndFiles(t *testing.T) {
	f := newFakeS3()
	for _, k := range []string{"p/", "p/a.txt", "p/b.txt", "p/sub/c.txt", "p/sub/deep/d.txt"} {
		f.put(k, []byte("x"))
	}
	cl := f.start(t)

	res, err := ListObjects(context.Background(), cl, "b", "p/", "", 100)
	if err != nil {
		t.Fatal(err)
	}

	var folders, files []string
	for _, e := range res.Entries {
		if e.IsFolder {
			folders = append(folders, e.Name)
		} else {
			files = append(files, e.Name)
		}
	}
	sort.Strings(folders)
	sort.Strings(files)

	if len(folders) != 1 || folders[0] != "sub" {
		t.Errorf("folders = %v, want [sub]", folders)
	}
	if len(files) != 2 || files[0] != "a.txt" || files[1] != "b.txt" {
		t.Errorf("files = %v, want [a.txt b.txt]", files)
	}
	// The folder's own placeholder object must not appear as a child.
	for _, e := range res.Entries {
		if e.Key == "p/" {
			t.Error("the prefix placeholder should be filtered out of its own listing")
		}
	}
}

func TestListAllObjectsFollowsPagination(t *testing.T) {
	f := newFakeS3()
	f.listPageMax = 2
	for _, k := range []string{"p/1", "p/2", "p/3", "p/4", "p/5", "other"} {
		f.put(k, []byte("x"))
	}
	cl := f.start(t)

	entries, err := ListAllObjects(context.Background(), cl, "b", "p/")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 5 {
		t.Fatalf("got %d entries across pages, want 5", len(entries))
	}
	for _, e := range entries {
		if e.Key == "other" {
			t.Error("listing leaked an object from outside the prefix")
		}
	}
}

func TestDeletePrefixRemovesEverythingUnderIt(t *testing.T) {
	f := newFakeS3()
	f.listPageMax = 2
	for _, k := range []string{"p/", "p/a", "p/b", "p/sub/c", "keep"} {
		f.put(k, []byte("x"))
	}
	cl := f.start(t)

	if err := DeletePrefix(context.Background(), cl, "b", "p/"); err != nil {
		t.Fatal(err)
	}
	if got := f.keys(); len(got) != 1 || got[0] != "keep" {
		t.Errorf("after DeletePrefix the bucket holds %v, want [keep]", got)
	}
}

func TestDeletePrefixOnEmptyFolderRemovesThePlaceholder(t *testing.T) {
	f := newFakeS3()
	f.put("empty/", nil)
	f.put("keep", []byte("x"))
	cl := f.start(t)

	if err := DeletePrefix(context.Background(), cl, "b", "empty/"); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.get("empty/"); ok {
		t.Error("the placeholder object for an empty folder should be deleted")
	}
	if _, ok := f.get("keep"); !ok {
		t.Error("unrelated objects must survive")
	}
}

func TestObjectSizesReportsPerKeyAndToleratesMisses(t *testing.T) {
	f := newFakeS3()
	f.put("a", bytes.Repeat([]byte("x"), 10))
	f.put("b", bytes.Repeat([]byte("x"), 250))
	cl := f.start(t)

	sizes := ObjectSizes(context.Background(), cl, "b", []string{"a", "b", "missing"})
	if sizes["a"] != 10 || sizes["b"] != 250 {
		t.Errorf("sizes = %v, want a=10 b=250", sizes)
	}
	if _, ok := sizes["missing"]; ok {
		t.Error("a key that cannot be headed should be absent, not zero-valued in a way that hides the miss")
	}
	if got := ObjectSizes(context.Background(), cl, "b", nil); len(got) != 0 {
		t.Errorf("empty input should return an empty map, got %v", got)
	}
}

func TestUploadSmallFileUsesSinglePut(t *testing.T) {
	f := newFakeS3()
	cl := f.start(t)

	dir := t.TempDir()
	local := filepath.Join(dir, "small.txt")
	payload := []byte("hello scalpel")
	if err := os.WriteFile(local, payload, 0o600); err != nil {
		t.Fatal(err)
	}

	var reported []int64
	err := Upload(context.Background(), cl, "b", "small.txt", local, true, MinPartSize,
		UploadOptions{}, func(n int64) { reported = append(reported, n) })
	if err != nil {
		t.Fatal(err)
	}

	got, ok := f.get("small.txt")
	if !ok || !bytes.Equal(got, payload) {
		t.Fatalf("uploaded %q, want %q", got, payload)
	}
	if len(reported) == 0 || reported[len(reported)-1] != int64(len(payload)) {
		t.Errorf("final progress = %v, want %d", reported, len(payload))
	}
	if f.nextUpload != 0 {
		t.Error("a file smaller than the part size must not start a multipart upload")
	}
}

func TestMultipartUploadAssemblesPartsInOrder(t *testing.T) {
	f := newFakeS3()
	cl := f.start(t)

	// Three parts: two full and a remainder, uploaded concurrently.
	payload := make([]byte, 2*MinPartSize+1234)
	if _, err := rand.Read(payload); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	local := filepath.Join(dir, "big.bin")
	if err := os.WriteFile(local, payload, 0o600); err != nil {
		t.Fatal(err)
	}

	// ProgressFunc is called concurrently by the part goroutines.
	var maxProgress atomic.Int64
	err := Upload(context.Background(), cl, "b", "big.bin", local, true, MinPartSize,
		UploadOptions{PartConcurrency: 3}, func(n int64) { storeMax(&maxProgress, n) })
	if err != nil {
		t.Fatal(err)
	}

	got, ok := f.get("big.bin")
	if !ok {
		t.Fatal("object was not created")
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("reassembled object differs from the source (%d vs %d bytes)", len(got), len(payload))
	}
	if maxProgress.Load() != int64(len(payload)) {
		t.Errorf("progress topped out at %d, want %d", maxProgress.Load(), len(payload))
	}
	if f.nextUpload != 1 {
		t.Errorf("started %d multipart uploads, want 1", f.nextUpload)
	}
}

func TestMultipartUploadLeavesUploadResumableOnFailure(t *testing.T) {
	f := newFakeS3()
	cl := f.start(t)
	f.failAllParts = true

	payload := make([]byte, 2*MinPartSize)
	local := filepath.Join(t.TempDir(), "flaky.bin")
	if err := os.WriteFile(local, payload, 0o600); err != nil {
		t.Fatal(err)
	}

	var reportedID string
	err := Upload(context.Background(), cl, "b", "flaky.bin", local, true, MinPartSize,
		UploadOptions{OnUploadID: func(id string) { reportedID = id }}, nil)
	if err == nil {
		t.Fatal("expected the upload to fail")
	}
	if _, ok := f.get("flaky.bin"); ok {
		t.Error("a failed multipart upload must not leave a partial object behind")
	}
	if reportedID == "" {
		t.Fatal("the upload id must be reported so the caller can resume or abort it")
	}
	// The contract is deliberately "leave it open": the caller decides between
	// resuming and aborting, and aborting here would make resume impossible.
	f.mu.Lock()
	pending := len(f.uploads)
	f.mu.Unlock()
	if pending != 1 {
		t.Errorf("%d uploads open, want the failed one left resumable", pending)
	}

	if err := AbortUpload(context.Background(), cl, "b", "flaky.bin", reportedID); err != nil {
		t.Fatalf("abort: %v", err)
	}
	f.mu.Lock()
	pending = len(f.uploads)
	f.mu.Unlock()
	if pending != 0 {
		t.Errorf("%d uploads still open after abort, want 0", pending)
	}
}

func TestMultipartUploadResumesFromStoredParts(t *testing.T) {
	f := newFakeS3()
	cl := f.start(t)

	payload := make([]byte, 3*MinPartSize)
	if _, err := rand.Read(payload); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(t.TempDir(), "resume.bin")
	if err := os.WriteFile(local, payload, 0o600); err != nil {
		t.Fatal(err)
	}

	// First attempt: part 1 lands, parts 2 and 3 are rejected.
	f.failPartNums[2] = true
	f.failPartNums[3] = true
	var uploadID string
	err := Upload(context.Background(), cl, "b", "resume.bin", local, true, MinPartSize,
		UploadOptions{PartConcurrency: 1, OnUploadID: func(id string) { uploadID = id }}, nil)
	if err == nil {
		t.Fatal("expected the first attempt to fail")
	}
	f.mu.Lock()
	stored := len(f.uploads[uploadID])
	f.mu.Unlock()
	if stored == 0 {
		t.Fatal("the first attempt should have stored at least one part")
	}
	partsBefore := f.partPuts

	// Second attempt resumes: only the parts that are missing get re-sent.
	f.mu.Lock()
	f.failPartNums = map[int32]bool{}
	f.mu.Unlock()
	err = Upload(context.Background(), cl, "b", "resume.bin", local, true, MinPartSize,
		UploadOptions{PartConcurrency: 3, ResumeUploadID: uploadID}, nil)
	if err != nil {
		t.Fatalf("resumed upload failed: %v", err)
	}

	got, ok := f.get("resume.bin")
	if !ok || !bytes.Equal(got, payload) {
		t.Fatalf("resumed upload produced %d bytes, want the original %d", len(got), len(payload))
	}
	resent := f.partPuts - partsBefore
	if resent != 3-stored {
		t.Errorf("re-sent %d parts, want %d (the %d already stored should be reused)", resent, 3-stored, stored)
	}
}

func TestResumeWithStaleUploadIDStartsFresh(t *testing.T) {
	f := newFakeS3()
	cl := f.start(t)

	payload := make([]byte, 2*MinPartSize)
	local := filepath.Join(t.TempDir(), "stale.bin")
	if err := os.WriteFile(local, payload, 0o600); err != nil {
		t.Fatal(err)
	}

	err := Upload(context.Background(), cl, "b", "stale.bin", local, true, MinPartSize,
		UploadOptions{ResumeUploadID: "upload-does-not-exist"}, nil)
	if err != nil {
		t.Fatalf("an unknown upload id should fall back to a fresh upload, got %v", err)
	}
	got, _ := f.get("stale.bin")
	if len(got) != len(payload) {
		t.Errorf("uploaded %d bytes, want %d", len(got), len(payload))
	}
}

func TestListAndAbortMultipartUploads(t *testing.T) {
	f := newFakeS3()
	cl := f.start(t)

	local := filepath.Join(t.TempDir(), "orphan.bin")
	if err := os.WriteFile(local, make([]byte, 2*MinPartSize), 0o600); err != nil {
		t.Fatal(err)
	}
	f.failPartNums[2] = true // part 1 lands, part 2 fails, leaving the upload open
	_ = Upload(context.Background(), cl, "b", "orphan.bin", local, true, MinPartSize,
		UploadOptions{PartConcurrency: 1}, nil)

	uploads, err := ListMultipartUploads(context.Background(), cl, "b", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(uploads) != 1 {
		t.Fatalf("found %d dangling uploads, want 1", len(uploads))
	}
	if uploads[0].Key != "orphan.bin" {
		t.Errorf("key = %q, want orphan.bin", uploads[0].Key)
	}
	if uploads[0].PartCount == 0 || uploads[0].Size == 0 {
		t.Errorf("upload should report the billable parts it holds, got %+v", uploads[0])
	}

	n, err := AbortUploads(context.Background(), cl, "b", uploads)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("aborted %d uploads, want 1", n)
	}
	after, err := ListMultipartUploads(context.Background(), cl, "b", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 0 {
		t.Errorf("%d uploads remain after the sweep, want 0", len(after))
	}
}

func TestFreeKeyNumbersAroundExistingObjects(t *testing.T) {
	f := newFakeS3()
	f.put("dir/report.pdf", []byte("x"))
	f.put("dir/report (1).pdf", []byte("x"))
	f.put(".gitignore", []byte("x"))
	cl := f.start(t)
	ctx := context.Background()

	got, err := FreeKey(ctx, cl, "b", "dir/report.pdf", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != "dir/report (2).pdf" {
		t.Errorf("FreeKey = %q, want dir/report (2).pdf", got)
	}

	got, err = FreeKey(ctx, cl, "b", "dir/fresh.pdf", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != "dir/fresh.pdf" {
		t.Errorf("an unused key should be returned unchanged, got %q", got)
	}

	// A leading dot is part of the name, not an extension.
	got, err = FreeKey(ctx, cl, "b", ".gitignore", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != ".gitignore (1)" {
		t.Errorf("FreeKey = %q, want \".gitignore (1)\"", got)
	}
}

func TestObjectExistsDistinguishesAbsenceFromFailure(t *testing.T) {
	f := newFakeS3()
	f.put("here", []byte("x"))
	cl := f.start(t)
	ctx := context.Background()

	if ok, err := ObjectExists(ctx, cl, "b", "here"); err != nil || !ok {
		t.Errorf("ObjectExists(here) = %v, %v; want true, nil", ok, err)
	}
	if ok, err := ObjectExists(ctx, cl, "b", "gone"); err != nil || ok {
		t.Errorf("ObjectExists(gone) = %v, %v; want false, nil", ok, err)
	}
}

func TestDownloadWritesAtomicallyAndCleansUp(t *testing.T) {
	f := newFakeS3()
	payload := []byte("some object body")
	f.put("obj.txt", payload)
	cl := f.start(t)

	dir := t.TempDir()
	dst := filepath.Join(dir, "nested", "obj.txt")
	if err := Download(context.Background(), cl, "b", "obj.txt", dst, DownloadOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("downloaded %q, want %q", got, payload)
	}
	if _, err := os.Stat(dst + ".part"); !os.IsNotExist(err) {
		t.Error("the .part scratch file should be gone after a successful download")
	}
}

func TestDownloadFailureLeavesNoFile(t *testing.T) {
	f := newFakeS3()
	cl := f.start(t)

	dir := t.TempDir()
	dst := filepath.Join(dir, "missing.txt")
	if err := Download(context.Background(), cl, "b", "missing.txt", dst, DownloadOptions{}, nil); err == nil {
		t.Fatal("expected an error for a missing object")
	}
	for _, p := range []string{dst, dst + ".part"} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s should not exist after a failed download", p)
		}
	}
}

func TestParallelDownloadReassemblesRanges(t *testing.T) {
	f := newFakeS3()
	payload := make([]byte, MinParallelDownloadSize+4096)
	if _, err := rand.Read(payload); err != nil {
		t.Fatal(err)
	}
	f.put("big.bin", payload)
	cl := f.start(t)

	dir := t.TempDir()
	dst := filepath.Join(dir, "big.bin")
	var maxProgress atomic.Int64
	err := Download(context.Background(), cl, "b", "big.bin", dst,
		DownloadOptions{Multipart: true, PartSize: MinPartSize, PartConcurrency: 4},
		func(n int64) { storeMax(&maxProgress, n) })
	if err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("ranged download produced %d bytes that differ from the %d-byte source", len(got), len(payload))
	}
	if f.rangeReqs < 2 {
		t.Errorf("served %d ranged GETs, want the object split across several", f.rangeReqs)
	}
	if maxProgress.Load() != int64(len(payload)) {
		t.Errorf("progress topped out at %d, want %d", maxProgress.Load(), len(payload))
	}
}

func TestSmallObjectSkipsParallelDownload(t *testing.T) {
	f := newFakeS3()
	f.put("small.bin", make([]byte, 1024))
	cl := f.start(t)

	dst := filepath.Join(t.TempDir(), "small.bin")
	err := Download(context.Background(), cl, "b", "small.bin", dst,
		DownloadOptions{Multipart: true, PartSize: MinPartSize, PartConcurrency: 4}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if f.rangeReqs != 0 {
		t.Errorf("served %d ranged GETs for a small object, want 0", f.rangeReqs)
	}
}

func TestStreamCopyMovesBytesBetweenClients(t *testing.T) {
	src := newFakeS3()
	dst := newFakeS3()
	payload := []byte("cross-account payload")
	src.put("from.txt", payload)

	srcClient := src.start(t)
	dstClient := dst.start(t)

	var progress int64
	err := StreamCopy(context.Background(), srcClient, dstClient, "b", "from.txt", "b2", "to.txt",
		MinPartSize, UploadOptions{}, func(n int64) { progress = n })
	if err != nil {
		t.Fatal(err)
	}
	got, ok := dst.get("to.txt")
	if !ok || !bytes.Equal(got, payload) {
		t.Fatalf("destination holds %q, want %q", got, payload)
	}
	if progress != int64(len(payload)) {
		t.Errorf("progress = %d, want %d", progress, len(payload))
	}
	if _, ok := src.get("from.txt"); !ok {
		t.Error("a copy must leave the source in place")
	}
}

func TestSearchObjectsMatchesNameAndHonoursCap(t *testing.T) {
	f := newFakeS3()
	f.listPageMax = 2
	for _, k := range []string{"p/report-1.pdf", "p/sub/report-2.pdf", "p/notes.txt", "p/sub/"} {
		f.put(k, []byte("x"))
	}
	cl := f.start(t)

	hits, truncated, err := SearchObjects(context.Background(), cl, "b", "p/", "REPORT", 10)
	if err != nil {
		t.Fatal(err)
	}
	if truncated {
		t.Error("a 10-result cap should not truncate 2 hits")
	}
	if len(hits) != 2 {
		t.Fatalf("got %d hits, want 2: %+v", len(hits), hits)
	}
	for _, h := range hits {
		if h.Name != h.Key {
			t.Errorf("search rows should carry the full key as the display name, got %q", h.Name)
		}
	}

	capped, truncated, err := SearchObjects(context.Background(), cl, "b", "p/", "report", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(capped) != 1 || !truncated {
		t.Errorf("cap of 1 gave %d hits truncated=%v, want 1/true", len(capped), truncated)
	}
}

func TestPrefixStatsAggregatesByStorageClass(t *testing.T) {
	f := newFakeS3()
	f.put("p/", nil)
	f.put("p/a", bytes.Repeat([]byte("x"), 100))
	f.put("p/b", bytes.Repeat([]byte("x"), 50))
	f.put("outside", bytes.Repeat([]byte("x"), 999))
	cl := f.start(t)

	stats, err := PrefixStats(context.Background(), cl, "b", "p/")
	if err != nil {
		t.Fatal(err)
	}
	if stats.ObjectCount != 2 {
		t.Errorf("object count = %d, want 2 (folder placeholders excluded)", stats.ObjectCount)
	}
	if stats.TotalSize != 150 {
		t.Errorf("total size = %d, want 150", stats.TotalSize)
	}
	if got := stats.ByStorageClass["STANDARD"]; got.Count != 2 || got.Size != 150 {
		t.Errorf("STANDARD bucket = %+v, want count 2 size 150", got)
	}
}

func TestStreamCopyLargeObjectUsesSequentialMultipart(t *testing.T) {
	src := newFakeS3()
	dst := newFakeS3()

	// Two full parts plus a remainder, larger than a single PutObject path.
	payload := make([]byte, 2*MinPartSize+777)
	if _, err := rand.Read(payload); err != nil {
		t.Fatal(err)
	}
	src.put("big.bin", payload)

	var maxProgress atomic.Int64
	err := StreamCopy(context.Background(), src.start(t), dst.start(t), "b", "big.bin", "b2", "big.bin",
		MinPartSize, UploadOptions{}, func(n int64) { storeMax(&maxProgress, n) })
	if err != nil {
		t.Fatal(err)
	}

	got, ok := dst.get("big.bin")
	if !ok {
		t.Fatal("destination object was not created")
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("streamed copy produced %d bytes differing from the %d-byte source", len(got), len(payload))
	}
	if dst.nextUpload != 1 {
		t.Errorf("started %d multipart uploads on the destination, want 1", dst.nextUpload)
	}
	if maxProgress.Load() != int64(len(payload)) {
		t.Errorf("progress topped out at %d, want %d", maxProgress.Load(), len(payload))
	}
}

func TestStreamCopyEmptyObject(t *testing.T) {
	src := newFakeS3()
	dst := newFakeS3()
	src.put("empty.bin", nil)

	err := StreamCopy(context.Background(), src.start(t), dst.start(t), "b", "empty.bin", "b2", "empty.bin",
		MinPartSize, UploadOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := dst.get("empty.bin")
	if !ok || len(got) != 0 {
		t.Errorf("destination holds %d bytes, want an empty object", len(got))
	}
}

// The fake server speaks plain http, which forces the SDK to hash (and therefore
// rewind) request bodies. This is the configuration a self-hosted MinIO install
// commonly runs, and a non-seekable body fails outright on it.
func TestUploadWorksAgainstPlainHTTPEndpoint(t *testing.T) {
	f := newFakeS3()
	cl := f.start(t)

	local := filepath.Join(t.TempDir(), "payload.bin")
	payload := bytes.Repeat([]byte("scalpel"), 1000)
	if err := os.WriteFile(local, payload, 0o600); err != nil {
		t.Fatal(err)
	}

	var last int64
	if err := Upload(context.Background(), cl, "b", "payload.bin", local, false, MinPartSize,
		UploadOptions{}, func(n int64) { last = n }); err != nil {
		t.Fatalf("upload over an http endpoint failed: %v", err)
	}
	got, _ := f.get("payload.bin")
	if !bytes.Equal(got, payload) {
		t.Error("uploaded bytes differ from the source file")
	}
	if last != int64(len(payload)) {
		t.Errorf("final progress = %d, want %d (the counter must track the transmitting pass)", last, len(payload))
	}
}
