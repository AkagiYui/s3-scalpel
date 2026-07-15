package model

// BucketVersioning is a bucket's versioning state. Status is "Enabled",
// "Suspended", or "" when versioning was never configured.
type BucketVersioning struct {
	Status    string `json:"status"`
	MFADelete bool   `json:"mfaDelete"`
}

// CORSRule is one cross-origin resource sharing rule.
type CORSRule struct {
	ID             string   `json:"id"`
	AllowedOrigins []string `json:"allowedOrigins"`
	AllowedMethods []string `json:"allowedMethods"`
	AllowedHeaders []string `json:"allowedHeaders"`
	ExposeHeaders  []string `json:"exposeHeaders"`
	MaxAgeSeconds  int32    `json:"maxAgeSeconds"`
}

// LifecycleRule is a simplified representation of an S3 lifecycle rule covering
// the fields a desktop client realistically edits.
type LifecycleRule struct {
	ID                              string `json:"id"`
	Prefix                          string `json:"prefix"`
	Enabled                         bool   `json:"enabled"`
	ExpirationDays                  int32  `json:"expirationDays"`
	NoncurrentVersionExpirationDays int32  `json:"noncurrentVersionExpirationDays"`
	AbortIncompleteMultipartDays    int32  `json:"abortIncompleteMultipartDays"`
	TransitionDays                  int32  `json:"transitionDays"`
	TransitionStorageClass          string `json:"transitionStorageClass"`
}

// BucketEncryption describes the bucket's default server-side encryption.
type BucketEncryption struct {
	Enabled          bool   `json:"enabled"`
	SSEAlgorithm     string `json:"sseAlgorithm"` // "AES256" | "aws:kms"
	KMSKeyID         string `json:"kmsKeyId"`
	BucketKeyEnabled bool   `json:"bucketKeyEnabled"`
}

// PublicAccessBlock mirrors the S3 public access block configuration. Configured
// is false when the bucket has no block configuration at all.
type PublicAccessBlock struct {
	Configured            bool `json:"configured"`
	BlockPublicAcls       bool `json:"blockPublicAcls"`
	IgnorePublicAcls      bool `json:"ignorePublicAcls"`
	BlockPublicPolicy     bool `json:"blockPublicPolicy"`
	RestrictPublicBuckets bool `json:"restrictPublicBuckets"`
}

// ACLGrant is one grantee/permission pair in an object ACL.
type ACLGrant struct {
	Grantee    string `json:"grantee"`
	Permission string `json:"permission"`
	URI        string `json:"uri"`
}

// ObjectACL is a simplified view of an object's access control list.
type ObjectACL struct {
	Owner    string     `json:"owner"`
	IsPublic bool       `json:"isPublic"`
	Grants   []ACLGrant `json:"grants"`
}

// StorageClassStat is the per-storage-class portion of a PrefixStats aggregate.
type StorageClassStat struct {
	Count int64 `json:"count"`
	Size  int64 `json:"size"`
}

// PrefixStats aggregates object count and cumulative size under a prefix.
type PrefixStats struct {
	Prefix         string                      `json:"prefix"`
	ObjectCount    int64                       `json:"objectCount"`
	TotalSize      int64                       `json:"totalSize"`
	ByStorageClass map[string]StorageClassStat `json:"byStorageClass"`
}

// SearchResult is a page of recursive search hits.
type SearchResult struct {
	Entries   []ObjectEntry `json:"entries"`
	Truncated bool          `json:"truncated"`
}

// ObjectMetaUpdate carries the editable system/user metadata rewritten onto an
// object via a self-copy.
type ObjectMetaUpdate struct {
	ContentType        string            `json:"contentType"`
	CacheControl       string            `json:"cacheControl"`
	ContentDisposition string            `json:"contentDisposition"`
	ContentEncoding    string            `json:"contentEncoding"`
	StorageClass       string            `json:"storageClass"`
	Metadata           map[string]string `json:"metadata"`
}
