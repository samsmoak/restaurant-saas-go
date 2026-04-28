package service_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"restaurantsaas/internal/apps/upload/service"
)

func TestCustomerPresignValidate_HappyPath(t *testing.T) {
	r := &service.CustomerPresignRequest{
		Filename:    "selfie.jpg",
		ContentType: "image/jpeg",
		Size:        2 * 1024 * 1024,
	}
	assert.NoError(t, r.Validate())
}

func TestCustomerPresignValidate_RejectsOver4MB(t *testing.T) {
	r := &service.CustomerPresignRequest{
		Filename:    "huge.png",
		ContentType: "image/png",
		Size:        5 * 1024 * 1024,
	}
	err := r.Validate()
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "4MB")
	}
}

func TestCustomerPresignValidate_RejectsNonImage(t *testing.T) {
	r := &service.CustomerPresignRequest{
		Filename:    "doc.pdf",
		ContentType: "application/pdf",
		Size:        1024,
	}
	err := r.Validate()
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "image/")
	}
}

func TestCustomerPresignValidate_RejectsZeroSize(t *testing.T) {
	r := &service.CustomerPresignRequest{
		Filename:    "selfie.jpg",
		ContentType: "image/jpeg",
		Size:        0,
	}
	assert.Error(t, r.Validate())
}

func TestCustomerPresignValidate_RejectsEmptyFilename(t *testing.T) {
	r := &service.CustomerPresignRequest{
		Filename:    "   ",
		ContentType: "image/jpeg",
		Size:        1024,
	}
	assert.Error(t, r.Validate())
}

// Existing admin-side request still enforces 8MB and prefix rules — sanity check.
func TestAdminPresignValidate_RejectsCustomerAvatarsPrefix(t *testing.T) {
	r := &service.PresignRequest{
		Prefix:      service.UploadPrefixCustomerAvatars,
		Filename:    "x.jpg",
		ContentType: "image/jpeg",
		Size:        1024,
	}
	err := r.Validate()
	if assert.Error(t, err) {
		// Admin presign restricts to logos + menu-images; customer-avatars is
		// only reachable via the /api/me/uploads/presign route.
		assert.Contains(t, err.Error(), "prefix")
	}
}
