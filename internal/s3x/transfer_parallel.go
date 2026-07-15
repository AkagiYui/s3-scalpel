package s3x

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// DefaultPartConcurrency is the number of parts uploaded/downloaded in parallel
// when the caller does not specify one.
const DefaultPartConcurrency = 4

// MinParallelDownloadSize is the size threshold above which downloads switch from
// a single streaming GET to parallel ranged GETs.
const MinParallelDownloadSize int64 = 16 * 1024 * 1024

// partPlan describes one contiguous slice of a file.
type partPlan struct {
	Number int32
	Offset int64
	Length int64
}

// planParts splits a total size into sequential parts of at most partSize bytes.
// The final part carries the remainder. partSize is clamped to MinPartSize.
func planParts(size, partSize int64) []partPlan {
	if partSize < MinPartSize {
		partSize = MinPartSize
	}
	var plans []partPlan
	var num int32 = 1
	for off := int64(0); off < size; off += partSize {
		length := partSize
		if off+length > size {
			length = size - off
		}
		plans = append(plans, partPlan{Number: num, Offset: off, Length: length})
		num++
	}
	if len(plans) == 0 { // zero-byte file: one empty part
		plans = append(plans, partPlan{Number: 1, Offset: 0, Length: 0})
	}
	return plans
}

func clampConcurrency(n, parts int) int {
	if n <= 0 {
		n = DefaultPartConcurrency
	}
	if n > parts {
		n = parts
	}
	if n < 1 {
		n = 1
	}
	return n
}

// multipartUploadParallel uploads a file's parts concurrently using ReadAt so no
// shared read cursor is needed. Progress is reported as the cumulative sum of
// bytes across all in-flight parts.
func multipartUploadParallel(ctx context.Context, cl *s3.Client, bucket, key string, f *os.File, size int64, ct string, partSize int64, opts UploadOptions, onProgress ProgressFunc) error {
	createIn := &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		ContentType: aws.String(ct),
	}
	opts.applyCreateMultipart(createIn)
	create, err := cl.CreateMultipartUpload(ctx, createIn)
	if err != nil {
		return err
	}
	uploadID := create.UploadId
	abort := func() {
		_, _ = cl.AbortMultipartUpload(context.Background(), &s3.AbortMultipartUploadInput{
			Bucket: aws.String(bucket), Key: aws.String(key), UploadId: uploadID,
		})
	}

	plans := planParts(size, partSize)
	conc := clampConcurrency(opts.PartConcurrency, len(plans))

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		mu        sync.Mutex
		completed = make([]types.CompletedPart, 0, len(plans))
		firstErr  error
		total     atomic.Int64
	)
	sem := make(chan struct{}, conc)
	var wg sync.WaitGroup

	for _, p := range plans {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(p partPlan) {
			defer wg.Done()
			defer func() { <-sem }()

			section := io.NewSectionReader(f, p.Offset, p.Length)
			part, err := cl.UploadPart(ctx, &s3.UploadPartInput{
				Bucket:        aws.String(bucket),
				Key:           aws.String(key),
				UploadId:      uploadID,
				PartNumber:    aws.Int32(p.Number),
				Body:          section,
				ContentLength: aws.Int64(p.Length),
			})
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
					cancel()
				}
				mu.Unlock()
				return
			}
			if onProgress != nil {
				onProgress(total.Add(p.Length))
			}
			mu.Lock()
			completed = append(completed, types.CompletedPart{ETag: part.ETag, PartNumber: aws.Int32(p.Number)})
			mu.Unlock()
		}(p)
	}
	wg.Wait()

	if firstErr != nil {
		abort()
		return firstErr
	}
	if ctx.Err() != nil {
		abort()
		return ctx.Err()
	}

	sort.Slice(completed, func(i, j int) bool {
		return aws.ToInt32(completed[i].PartNumber) < aws.ToInt32(completed[j].PartNumber)
	})
	_, err = cl.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(bucket),
		Key:             aws.String(key),
		UploadId:        uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{Parts: completed},
	})
	if err != nil {
		abort()
		return err
	}
	return nil
}

// downloadParallel fetches an object using concurrent ranged GETs, writing each
// range at its offset. It assumes the caller has determined size > 0.
func downloadParallel(ctx context.Context, cl *s3.Client, bucket, key, dst string, size, partSize int64, concurrency int, onProgress ProgressFunc) error {
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	// Preallocate so concurrent WriteAt calls never race to extend the file.
	if err := f.Truncate(size); err != nil {
		f.Close()
		return err
	}

	plans := planParts(size, partSize)
	conc := clampConcurrency(concurrency, len(plans))

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		mu       sync.Mutex
		firstErr error
		total    atomic.Int64
	)
	sem := make(chan struct{}, conc)
	var wg sync.WaitGroup

	fail := func(err error) {
		mu.Lock()
		if firstErr == nil {
			firstErr = err
			cancel()
		}
		mu.Unlock()
	}

	for _, p := range plans {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(p partPlan) {
			defer wg.Done()
			defer func() { <-sem }()

			rng := fmt.Sprintf("bytes=%d-%d", p.Offset, p.Offset+p.Length-1)
			out, err := cl.GetObject(ctx, &s3.GetObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
				Range:  aws.String(rng),
			})
			if err != nil {
				fail(err)
				return
			}
			defer out.Body.Close()

			buf := make([]byte, 256*1024)
			offset := p.Offset
			for {
				if ctx.Err() != nil {
					fail(ctx.Err())
					return
				}
				n, rerr := out.Body.Read(buf)
				if n > 0 {
					if _, werr := f.WriteAt(buf[:n], offset); werr != nil {
						fail(werr)
						return
					}
					offset += int64(n)
					if onProgress != nil {
						onProgress(total.Add(int64(n)))
					}
				}
				if rerr == io.EOF {
					break
				}
				if rerr != nil {
					fail(rerr)
					return
				}
			}
		}(p)
	}
	wg.Wait()

	closeErr := f.Close()
	if firstErr != nil {
		os.Remove(dst)
		return firstErr
	}
	if ctx.Err() != nil {
		os.Remove(dst)
		return ctx.Err()
	}
	if closeErr != nil {
		os.Remove(dst)
		return closeErr
	}
	return nil
}
