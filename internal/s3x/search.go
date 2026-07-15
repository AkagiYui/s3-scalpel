package s3x

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"s3scalpel/internal/model"
)

// SearchObjects recursively scans every object under prefix and returns those
// whose key contains query (case-insensitive), up to maxResults. The bool result
// reports whether the scan stopped early because the cap was reached.
func SearchObjects(ctx context.Context, cl *s3.Client, bucket, prefix, query string, maxResults int) ([]model.ObjectEntry, bool, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	if maxResults <= 0 {
		maxResults = 1000
	}
	p := s3.NewListObjectsV2Paginator(cl, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	var results []model.ObjectEntry
	for p.HasMorePages() {
		if ctx.Err() != nil {
			return results, true, ctx.Err()
		}
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, false, err
		}
		for _, obj := range page.Contents {
			key := aws.ToString(obj.Key)
			if strings.HasSuffix(key, "/") {
				continue // skip folder placeholders
			}
			name := key
			if idx := strings.LastIndex(strings.TrimSuffix(key, "/"), "/"); idx >= 0 {
				name = key[idx+1:]
			}
			if q != "" && !strings.Contains(strings.ToLower(name), q) {
				continue
			}
			results = append(results, model.ObjectEntry{
				Key:          key,
				Name:         key, // full key so the row is unambiguous across folders
				Size:         aws.ToInt64(obj.Size),
				LastModified: ms(obj.LastModified),
				ETag:         strings.Trim(aws.ToString(obj.ETag), "\""),
				StorageClass: string(obj.StorageClass),
			})
			if len(results) >= maxResults {
				return results, true, nil
			}
		}
	}
	return results, false, nil
}

// PrefixStats aggregates the object count, cumulative size and per-storage-class
// breakdown for everything under a prefix.
func PrefixStats(ctx context.Context, cl *s3.Client, bucket, prefix string) (model.PrefixStats, error) {
	p := s3.NewListObjectsV2Paginator(cl, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	stats := model.PrefixStats{Prefix: prefix, ByStorageClass: map[string]model.StorageClassStat{}}
	for p.HasMorePages() {
		if ctx.Err() != nil {
			return stats, ctx.Err()
		}
		page, err := p.NextPage(ctx)
		if err != nil {
			return model.PrefixStats{}, err
		}
		for _, obj := range page.Contents {
			key := aws.ToString(obj.Key)
			if strings.HasSuffix(key, "/") {
				continue
			}
			size := aws.ToInt64(obj.Size)
			stats.ObjectCount++
			stats.TotalSize += size
			sc := string(obj.StorageClass)
			if sc == "" {
				sc = "STANDARD"
			}
			cur := stats.ByStorageClass[sc]
			cur.Count++
			cur.Size += size
			stats.ByStorageClass[sc] = cur
		}
	}
	return stats, nil
}
