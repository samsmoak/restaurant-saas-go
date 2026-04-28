package service_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"restaurantsaas/internal/apps/user/service"
)

// The service writes `photo_url` to Mongo only when the field is present
// in the JSON body. nil means "don't touch" so callers can update other
// fields without clobbering the avatar. These shape-level tests protect
// the JSON contract the customer apps rely on.

func TestProfileUpdateRequest_JSONOmittingPhotoURLLeavesItNil(t *testing.T) {
	body := []byte(`{"full_name":"Sam","phone":"5551234567"}`)
	var req service.ProfileUpdateRequest
	assert.NoError(t, json.Unmarshal(body, &req))
	assert.Nil(t, req.PhotoURL)
	assert.NoError(t, req.Validate())
}

func TestProfileUpdateRequest_JSONWithPhotoURLPopulatesPointer(t *testing.T) {
	body := []byte(`{"full_name":"Sam","phone":"5551234567","photo_url":"https://cdn.example/a.jpg"}`)
	var req service.ProfileUpdateRequest
	assert.NoError(t, json.Unmarshal(body, &req))
	if assert.NotNil(t, req.PhotoURL) {
		assert.Equal(t, "https://cdn.example/a.jpg", *req.PhotoURL)
	}
}

func TestProfileUpdateRequest_JSONWithEmptyPhotoURLClearsIt(t *testing.T) {
	// Sending `"photo_url": ""` is the explicit "clear my avatar" signal —
	// distinct from omitting the field entirely.
	body := []byte(`{"full_name":"Sam","phone":"5551234567","photo_url":""}`)
	var req service.ProfileUpdateRequest
	assert.NoError(t, json.Unmarshal(body, &req))
	if assert.NotNil(t, req.PhotoURL) {
		assert.Equal(t, "", *req.PhotoURL)
	}
}

func TestProfileUpdateRequest_RejectsShortName(t *testing.T) {
	body := []byte(`{"full_name":"S","phone":"5551234567"}`)
	var req service.ProfileUpdateRequest
	assert.NoError(t, json.Unmarshal(body, &req))
	assert.Error(t, req.Validate())
}
