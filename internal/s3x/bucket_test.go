package s3x

import (
	"errors"
	"testing"

	"github.com/aws/smithy-go"
)

func TestIsNoSuchConfig(t *testing.T) {
	absent := []string{
		"NoSuchBucketPolicy",
		"NoSuchCORSConfiguration",
		"NoSuchLifecycleConfiguration",
		"ServerSideEncryptionConfigurationNotFoundError",
		"NoSuchPublicAccessBlockConfiguration",
		"NoSuchTagSet",
	}
	for _, code := range absent {
		err := &smithy.GenericAPIError{Code: code, Message: "x"}
		if !isNoSuchConfig(err) {
			t.Errorf("isNoSuchConfig(%s) = false, want true", code)
		}
	}

	present := []error{
		nil,
		errors.New("boom"),
		&smithy.GenericAPIError{Code: "AccessDenied", Message: "nope"},
	}
	for _, err := range present {
		if isNoSuchConfig(err) {
			t.Errorf("isNoSuchConfig(%v) = true, want false", err)
		}
	}
}
