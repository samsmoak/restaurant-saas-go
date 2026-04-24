package model

import (
	"errors"
	"net/mail"
	"strings"
)

type SignupRequest struct {
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

func (r *SignupRequest) Validate() error {
	if len([]rune(strings.TrimSpace(r.FullName))) < 2 {
		return errors.New("full_name must be at least 2 characters")
	}
	if !IsEmail(r.Email) {
		return errors.New("email is invalid")
	}
	if len(strings.TrimSpace(r.Phone)) < 10 {
		return errors.New("phone must be at least 10 characters")
	}
	if len(r.Password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	return nil
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r *LoginRequest) Validate() error {
	if !IsEmail(r.Email) {
		return errors.New("email is invalid")
	}
	if len(r.Password) < 6 {
		return errors.New("password must be at least 6 characters")
	}
	return nil
}

type GoogleAuthRequest struct {
	IDToken string `json:"id_token"`
}

func (r *GoogleAuthRequest) Validate() error {
	if strings.TrimSpace(r.IDToken) == "" {
		return errors.New("id_token is required")
	}
	return nil
}

type EmailAvailableRequest struct {
	Email string `json:"email"`
}

func (r *EmailAvailableRequest) Validate() error {
	if !IsEmail(r.Email) {
		return errors.New("invalid email")
	}
	return nil
}

type EmailAvailableResponse struct {
	Available    bool   `json:"available"`
	RegisteredAs string `json:"registered_as,omitempty"`
}

type AdminFinalizeRequest struct {
	InviteCode string `json:"invite_code"`
}

func (r *AdminFinalizeRequest) Validate() error {
	if strings.TrimSpace(r.InviteCode) == "" {
		return errors.New("invite_code is required")
	}
	return nil
}

func IsEmail(s string) bool {
	addr, err := mail.ParseAddress(strings.TrimSpace(s))
	if err != nil {
		return false
	}
	return addr.Address != "" && strings.Contains(addr.Address, "@")
}
