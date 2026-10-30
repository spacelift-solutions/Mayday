package license

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/spacelift-solutions/Mayday/internal/model"
)

var ErrMissing = errors.New("license object missing")
var ErrInvalid = errors.New("license content invalid")
var ErrDenied = errors.New("license access denied")

type Store interface {
	Check(context.Context) error
	Delete(context.Context) error
}

func validate(r io.Reader) error {
	var l model.License
	if err := json.NewDecoder(io.LimitReader(r, 4096)).Decode(&l); err != nil {
		return ErrInvalid
	}
	if l.Service != "mayday" || l.Key != "DEMO-ONLY" || !l.Enabled {
		return ErrInvalid
	}
	return nil
}

type File struct{ Path string }

func (f File) Check(_ context.Context) error {
	r, err := os.Open(f.Path)
	if errors.Is(err, os.ErrNotExist) {
		return ErrMissing
	}
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()
	return validate(r)
}
func (f File) Delete(_ context.Context) error {
	err := os.Remove(f.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

type S3 struct {
	Client      *s3.Client
	Bucket, Key string
}

func (s S3) Check(ctx context.Context) error {
	out, err := s.Client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.Bucket), Key: aws.String(s.Key)})
	if err != nil {
		var api smithy.APIError
		if errors.As(err, &api) {
			switch api.ErrorCode() {
			case "NoSuchKey", "NotFound":
				return ErrMissing
			case "AccessDenied":
				return ErrDenied
			}
		}
		return fmt.Errorf("S3 dependency check: %w", err)
	}
	defer func() { _ = out.Body.Close() }()
	return validate(out.Body)
}
func (s S3) Delete(ctx context.Context) error {
	_, err := s.Client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.Bucket), Key: aws.String(s.Key)})
	return err
}

func Status(err error) string {
	switch {
	case err == nil:
		return "valid"
	case errors.Is(err, ErrMissing):
		return "missing"
	case errors.Is(err, ErrInvalid):
		return "invalid"
	case errors.Is(err, ErrDenied):
		return "access_denied"
	default:
		return "unavailable"
	}
}
