// Package model holds the data types shared between the Go backend and the
// (generated) TypeScript frontend bindings. Keep these JSON-tagged and free of
// behaviour so the binding generator produces clean models.
package model

// Connection is a single S3-compatible account configuration. It deliberately
// does NOT include a bucket name: buckets are listed after connecting.
type Connection struct {
	ID           string `json:"id"`
	Name         string `json:"name"`         // display name
	Endpoint     string `json:"endpoint"`     // e.g. https://cn-nb1.example.com
	Region       string `json:"region"`       // SigV4 region; "auto" for R2
	PathStyle    bool   `json:"pathStyle"`    // true = path-style (MinIO), false = virtual-hosted
	AccessKey    string `json:"accessKey"`
	SecretKey    string `json:"secretKey"`
	SessionToken string `json:"sessionToken"` // optional STS temporary-credential token

	// Transport options for self-hosted / restricted endpoints.
	SkipTLSVerify bool   `json:"skipTlsVerify"` // do not verify the server certificate
	ProxyURL      string `json:"proxyUrl"`      // optional HTTP/HTTPS/SOCKS proxy URL
	CACert        string `json:"caCert"`        // optional PEM CA bundle to trust

	CreatedAt int64 `json:"createdAt"`
}

// AppSettings is the persisted application configuration. Shared across windows.
type AppSettings struct {
	Language           string `json:"language"`           // "system" | "zh" | "en"
	Theme              string `json:"theme"`              // "system" | "light" | "dark"
	NotifyEnabled      bool   `json:"notifyEnabled"`      // send system notifications
	NotifySound        bool   `json:"notifySound"`        // play a sound cue
	Concurrency        int    `json:"concurrency"`        // queue parallelism (default 5)
	PartSize           int64  `json:"partSize"`           // multipart chunk size in bytes (default 8MiB, min 5MiB)
	MultipartEnabled   bool   `json:"multipartEnabled"`   // use multipart upload for large files
	AutoConsumeQueue   bool   `json:"autoConsumeQueue"`   // automatically run queued tasks
	PreviewMaxSize     int64  `json:"previewMaxSize"`     // max bytes to download for preview
	DefaultDownloadDir string `json:"defaultDownloadDir"` // remembered download directory

	UploadStorageClass string `json:"uploadStorageClass"` // default storage class for uploads ("" = provider default)
	UploadSSE          string `json:"uploadSSE"`          // "", "AES256", "aws:kms"
	UploadKMSKeyID     string `json:"uploadKMSKeyId"`     // KMS key id when UploadSSE is "aws:kms"
	PartConcurrency    int    `json:"partConcurrency"`    // concurrent parts per multipart transfer (default 4)
}

// BucketInfo describes a bucket in a connection.
type BucketInfo struct {
	Name      string `json:"name"`
	CreatedAt int64  `json:"createdAt"`
}

// ObjectEntry is one row in a folder-style object listing.
type ObjectEntry struct {
	Key          string `json:"key"`          // full object key (folders end with "/")
	Name         string `json:"name"`         // last path segment for display
	IsFolder     bool   `json:"isFolder"`     // derived from CommonPrefixes
	Size         int64  `json:"size"`         //
	LastModified int64  `json:"lastModified"` // unix milliseconds
	ETag         string `json:"etag"`
	StorageClass string `json:"storageClass"`
}

// ListResult is a single page of a listing.
type ListResult struct {
	Prefix      string        `json:"prefix"`
	Entries     []ObjectEntry `json:"entries"`
	NextToken   string        `json:"nextToken"`
	IsTruncated bool          `json:"isTruncated"`
}

// ObjectProperties is the full metadata returned by HeadObject.
type ObjectProperties struct {
	Key                string            `json:"key"`
	Size               int64             `json:"size"`
	ContentType        string            `json:"contentType"`
	ETag               string            `json:"etag"`
	LastModified       int64             `json:"lastModified"`
	StorageClass       string            `json:"storageClass"`
	VersionID          string            `json:"versionId"`
	CacheControl       string            `json:"cacheControl"`
	ContentEncoding    string            `json:"contentEncoding"`
	ContentDisposition string            `json:"contentDisposition"`
	Metadata           map[string]string `json:"metadata"`
}

// Tag is an object tag key/value pair.
type Tag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ObjectVersion is one entry from ListObjectVersions.
type ObjectVersion struct {
	Key            string `json:"key"`
	VersionID      string `json:"versionId"`
	IsLatest       bool   `json:"isLatest"`
	Size           int64  `json:"size"`
	LastModified   int64  `json:"lastModified"`
	ETag           string `json:"etag"`
	IsDeleteMarker bool   `json:"isDeleteMarker"`
}

// TaskType enumerates queued operation kinds.
type TaskType string

const (
	TaskUpload   TaskType = "upload"
	TaskDownload TaskType = "download"
	TaskDelete   TaskType = "delete"
	TaskCopy     TaskType = "copy"
	TaskMove     TaskType = "move"
)

// TaskStatus enumerates the lifecycle states of a task.
type TaskStatus string

const (
	StatusPending   TaskStatus = "pending"
	StatusRunning   TaskStatus = "running"
	StatusCompleted TaskStatus = "completed"
	StatusFailed    TaskStatus = "failed"
	StatusCanceled  TaskStatus = "canceled"
)

// Task is a unit of work in a window's operation queue. While running, only its
// status/progress fields are meaningful to persist; transient buffers are not
// stored. On an unclean shutdown, running tasks are reloaded as failed.
type Task struct {
	ID           string     `json:"id"`
	WindowID     string     `json:"windowId"`
	Type         TaskType   `json:"type"`
	Status       TaskStatus `json:"status"`
	ConnectionID string     `json:"connectionId"`
	Bucket       string     `json:"bucket"`
	Key          string     `json:"key"`            // primary/source object key
	DestConnID   string     `json:"destConnId"`     // copy/move target connection (empty = same as ConnectionID)
	DestBucket   string     `json:"destBucket"`     // copy/move target
	DestKey      string     `json:"destKey"`
	LocalPath    string     `json:"localPath"` // up/download local file path
	Size         int64      `json:"size"`
	Transferred  int64      `json:"transferred"`
	Priority     int        `json:"priority"` // higher runs sooner
	Error        string     `json:"error"`
	Retries      int        `json:"retries"`
	CreatedAt    int64      `json:"createdAt"`
	UpdatedAt    int64      `json:"updatedAt"`
}

// PreviewKind tells the frontend how to render a preview payload.
type PreviewKind string

const (
	PreviewImage       PreviewKind = "image"
	PreviewText        PreviewKind = "text"
	PreviewPDF         PreviewKind = "pdf"
	PreviewMedia       PreviewKind = "media" // audio/video, streamed via URL
	PreviewUnsupported PreviewKind = "unsupported"
	PreviewTooLarge    PreviewKind = "too-large"
)

// PreviewData is returned to the frontend for object preview.
type PreviewData struct {
	Kind        PreviewKind `json:"kind"`
	ContentType string      `json:"contentType"`
	Size        int64       `json:"size"`
	DataURL     string      `json:"dataUrl"` // image/pdf: data: URL
	Text        string      `json:"text"`    // text: raw content
	URL         string      `json:"url"`     // media: presigned URL
}

// TestResult is returned by a connection test.
type TestResult struct {
	OK          bool   `json:"ok"`
	Message     string `json:"message"`
	BucketCount int    `json:"bucketCount"`
}

// Capability is the result of probing a single S3 operation to discover whether
// the credentials are permitted to perform it. S3 endpoints expose no API to
// query permissions, so they are determined empirically.
type Capability struct {
	Op      string `json:"op"`      // stable operation id, e.g. "putObject"
	Allowed bool   `json:"allowed"` // whether the probe succeeded
	Tested  bool   `json:"tested"`  // whether the probe ran (bucket ops need a bucket)
	Detail  string `json:"detail"`  // short explanation on failure
}
