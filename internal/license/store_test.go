package license

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/spacelift-solutions/Mayday/internal/model"
)

func TestLicenseValidation(t *testing.T) {
	for _, tc := range []struct {
		body  string
		valid bool
	}{
		{`{"service":"mayday","license_key":"DEMO-ONLY","enabled":true}`, true},
		{`{"service":"other","license_key":"DEMO-ONLY","enabled":true}`, false},
		{`{"service":"mayday","license_key":"DEMO-ONLY","enabled":false}`, false},
		{`{"service":"mayday","license_key":"wrong","enabled":true}`, false},
		{`not json`, false},
	} {
		if (validate(strings.NewReader(tc.body)) == nil) != tc.valid {
			t.Errorf("unexpected validation for %s", tc.body)
		}
	}
}

func TestFileInvalidationCanBeRepairedRepeatedly(t *testing.T) {
	ctx := context.Background()
	store := File{Path: filepath.Join(t.TempDir(), "license.json")}
	valid := []byte("{\"service\":\"mayday\",\"license_key\":\"DEMO-ONLY\",\"enabled\":true}\n")
	for cycle := 0; cycle < 2; cycle++ {
		if err := os.WriteFile(store.Path, valid, 0600); err != nil {
			t.Fatal(err)
		}
		if err := store.Check(ctx); err != nil {
			t.Fatalf("restored license: %v", err)
		}
		if err := store.Invalidate(ctx); err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(store.Path)
		if err != nil {
			t.Fatalf("fault removed the license file: %v", err)
		}
		var content model.License
		if err := json.Unmarshal(body, &content); err != nil || content.Service != "mayday" || content.Key != "DEMO-ONLY" || content.Enabled {
			t.Fatalf("fault must disable a recognizable license: %s (%v)", body, err)
		}
		if err := store.Check(ctx); !errors.Is(err, ErrInvalid) {
			t.Fatalf("expected invalid license, got %v", err)
		}
	}
}

func TestS3InvalidationWritesScopedEncryptedObject(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut || r.URL.Path != "/demo-bucket/license/demo-license.json" {
					t.Errorf("unexpected operation: %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("X-Amz-Server-Side-Encryption") != "AES256" || r.Header.Get("Content-Type") != "application/json" {
					t.Error("missing encryption or JSON content type")
				}
				var content model.License
				if err := json.NewDecoder(r.Body).Decode(&content); err != nil || content.Service != "mayday" || content.Key != "DEMO-ONLY" || content.Enabled {
					t.Errorf("invalid fault payload: %+v (%v)", content, err)
				}
				w.WriteHeader(status)
				if status == http.StatusForbidden {
					_, _ = io.WriteString(w, "<Error><Code>AccessDenied</Code></Error>")
				}
			}))
			defer server.Close()
			client := s3.New(s3.Options{
				Region: "us-east-1", BaseEndpoint: aws.String(server.URL),
				Credentials: aws.AnonymousCredentials{}, UsePathStyle: true,
				RetryMaxAttempts: 1,
			})
			store := S3{Client: client, Bucket: "demo-bucket", Key: "license/demo-license.json"}
			err := store.Invalidate(context.Background())
			if (err == nil) != (status == http.StatusOK) {
				t.Fatalf("status %d returned %v", status, err)
			}
		})
	}
}
