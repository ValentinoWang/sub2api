package membership

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type S3AuditSink struct {
	Client *s3.Client
	Bucket string
}

func (s *S3AuditSink) Store(ctx context.Context, id string, raw []byte) (string, error) {
	if s == nil || s.Client == nil || s.Bucket == "" {
		return "", ErrUnavailable
	}
	sum := sha256.Sum256(raw)
	key := "membership/" + id + "/" + hex.EncodeToString(sum[:]) + ".json"
	retained := time.Now().UTC().AddDate(7, 0, 0)
	_, err := s.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.Bucket), Key: aws.String(key), Body: bytes.NewReader(raw),
		ContentType: aws.String("application/json"), ObjectLockMode: types.ObjectLockModeCompliance,
		ObjectLockRetainUntilDate: &retained,
	})
	if err != nil {
		return "", ErrUnavailable
	}
	head, err := s.Client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.Bucket), Key: aws.String(key)})
	if err != nil || head.ObjectLockMode != types.ObjectLockModeCompliance || head.ObjectLockRetainUntilDate == nil || head.ObjectLockRetainUntilDate.Before(retained.Add(-time.Minute)) {
		return "", ErrUnavailable
	}
	return key, nil
}
