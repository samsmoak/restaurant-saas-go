package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	s3util "restaurantsaas/internal/s3"
)

const (
	UploadPrefixLogos            = "logos"
	UploadPrefixMenuImages       = "menu-images"
	UploadPrefixCustomerAvatars  = "customer-avatars"
	MaxUploadBytes               = 8 * 1024 * 1024
	MaxCustomerAvatarUploadBytes = 4 * 1024 * 1024
)

type PresignRequest struct {
	Prefix      string `json:"prefix"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

func (r *PresignRequest) Validate() error {
	if r.Prefix != UploadPrefixLogos && r.Prefix != UploadPrefixMenuImages {
		return errors.New("prefix must be 'logos' or 'menu-images'")
	}
	if !strings.HasPrefix(r.ContentType, "image/") {
		return errors.New("content_type must be an image/* type")
	}
	if r.Size <= 0 {
		return errors.New("size is required")
	}
	if r.Size > MaxUploadBytes {
		return errors.New("file too large (max 8MB)")
	}
	if strings.TrimSpace(r.Filename) == "" {
		return errors.New("filename is required")
	}
	return nil
}

// CustomerPresignRequest is the customer-app payload for profile photo
// uploads. It enforces a 4MB cap and the customer-avatars prefix.
type CustomerPresignRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

func (r *CustomerPresignRequest) Validate() error {
	if !strings.HasPrefix(r.ContentType, "image/") {
		return errors.New("content_type must be an image/* type")
	}
	if r.Size <= 0 {
		return errors.New("size is required")
	}
	if r.Size > MaxCustomerAvatarUploadBytes {
		return errors.New("file too large (max 4MB)")
	}
	if strings.TrimSpace(r.Filename) == "" {
		return errors.New("filename is required")
	}
	return nil
}

type PresignResult struct {
	UploadURL string `json:"upload_url"`
	PublicURL string `json:"public_url"`
	Key       string `json:"key"`
}

type DirectResult struct {
	PublicURL string `json:"public_url"`
	Key       string `json:"key"`
}

type UploadService interface {
	Presign(ctx context.Context, req *PresignRequest) (*PresignResult, error)
	Direct(ctx context.Context, prefix, filename, contentType string, size int64, body []byte) (*DirectResult, error)
	PresignCustomerAvatar(ctx context.Context, req *CustomerPresignRequest) (*PresignResult, error)
}

type uploadService struct{}

func NewUploadService() UploadService { return &uploadService{} }

func (s *uploadService) Presign(ctx context.Context, req *PresignRequest) (*PresignResult, error) {
	if !s3util.IsConfigured() {
		return nil, errors.New("S3 is not configured")
	}
	key := s3util.BuildObjectKey(req.Prefix, req.Filename)
	uploadURL, err := s3util.PresignedPutURL(ctx, key, req.ContentType)
	if err != nil {
		return nil, fmt.Errorf("UploadService.Presign: %w", err)
	}
	return &PresignResult{
		UploadURL: uploadURL,
		PublicURL: s3util.PublicURLFor(key),
		Key:       key,
	}, nil
}

func (s *uploadService) PresignCustomerAvatar(ctx context.Context, req *CustomerPresignRequest) (*PresignResult, error) {
	if !s3util.IsConfigured() {
		return nil, errors.New("S3 is not configured")
	}
	key := s3util.BuildObjectKey(UploadPrefixCustomerAvatars, req.Filename)
	uploadURL, err := s3util.PresignedPutURL(ctx, key, req.ContentType)
	if err != nil {
		return nil, fmt.Errorf("UploadService.PresignCustomerAvatar: %w", err)
	}
	return &PresignResult{
		UploadURL: uploadURL,
		PublicURL: s3util.PublicURLFor(key),
		Key:       key,
	}, nil
}

func (s *uploadService) Direct(ctx context.Context, prefix, filename, contentType string, size int64, body []byte) (*DirectResult, error) {
	if !s3util.IsConfigured() {
		return nil, errors.New("S3 is not configured")
	}
	pr := &PresignRequest{Prefix: prefix, Filename: filename, ContentType: contentType, Size: size}
	if err := pr.Validate(); err != nil {
		return nil, err
	}
	key := s3util.BuildObjectKey(prefix, filename)
	if err := s3util.PutObject(ctx, key, body, contentType); err != nil {
		return nil, fmt.Errorf("UploadService.Direct: %w", err)
	}
	return &DirectResult{PublicURL: s3util.PublicURLFor(key), Key: key}, nil
}
