package s3util

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	client  *s3.Client
	presign *s3.PresignClient
)

func IsConfigured() bool {
	return os.Getenv("AWS_S3_BUCKET") != "" &&
		os.Getenv("AWS_S3_REGION") != "" &&
		os.Getenv("AWS_ACCESS_KEY_ID") != "" &&
		os.Getenv("AWS_SECRET_ACCESS_KEY") != "" &&
		os.Getenv("AWS_ACCESS_KEY_ID") != "your_new_rotated_access_key"
}

func Client(ctx context.Context) (*s3.Client, error) {
	if client != nil {
		return client, nil
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(os.Getenv("AWS_S3_REGION")),
	)
	if err != nil {
		return nil, fmt.Errorf("s3util.Client: %w", err)
	}
	client = s3.NewFromConfig(cfg)
	presign = s3.NewPresignClient(client)
	return client, nil
}

func BuildObjectKey(prefix, filename string) string {
	ext := "bin"
	if i := strings.LastIndex(filename, "."); i >= 0 && i+1 < len(filename) {
		raw := strings.ToLower(filename[i+1:])
		clean := stripNonAlphanum(raw)
		if len(clean) > 0 && len(clean) <= 5 {
			ext = clean
		}
	}
	suffix := randomSuffix(6)
	return fmt.Sprintf("%s/%d-%s.%s", prefix, time.Now().UnixMilli(), suffix, ext)
}

func PublicURLFor(key string) string {
	base := strings.TrimRight(os.Getenv("S3_PUBLIC_BASE_URL"), "/")
	if base != "" {
		return base + "/" + key
	}
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s",
		os.Getenv("AWS_S3_BUCKET"), os.Getenv("AWS_S3_REGION"), key)
}

func PutObject(ctx context.Context, key string, body []byte, contentType string) error {
	c, err := Client(ctx)
	if err != nil {
		return err
	}
	_, err = c.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(os.Getenv("AWS_S3_BUCKET")),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("s3util.PutObject: %w", err)
	}
	return nil
}

func PresignedPutURL(ctx context.Context, key, contentType string) (string, error) {
	if _, err := Client(ctx); err != nil {
		return "", err
	}
	req, err := presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(os.Getenv("AWS_S3_BUCKET")),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(60*time.Second))
	if err != nil {
		return "", fmt.Errorf("s3util.PresignedPutURL: %w", err)
	}
	return req.URL, nil
}

func stripNonAlphanum(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func randomSuffix(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	out := make([]byte, n)
	for i := range out {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			out[i] = chars[0]
			continue
		}
		out[i] = chars[idx.Int64()]
	}
	return string(out)
}
