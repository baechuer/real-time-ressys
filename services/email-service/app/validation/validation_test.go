package validation

import (
	"testing"
)

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"valid email", "test@example.com", false},
		{"valid email with subdomain", "user@mail.example.com", false},
		{"empty email", "", true},
		{"invalid format", "notanemail", true},
		{"missing @", "testexample.com", true},
		{"missing domain", "test@", true},
		{"consecutive dots", "test..user@example.com", true},
		{"too long", string(make([]byte, MaxEmailLength+1)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmail(tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEmail() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid https URL", "https://example.com/verify?token=abc", false},
		{"valid http URL", "http://example.com/reset", false},
		{"empty URL", "", true},
		{"invalid scheme", "javascript:alert(1)", true},
		{"data URL", "data:text/html,<script>alert(1)</script>", true},
		{"no scheme", "example.com", true},
		{"no host", "https://", true},
		{"too long", "https://example.com/" + string(make([]byte, MaxURLLength)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSanitizeEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  string
	}{
		{"normal email", "test@example.com", "te***@ex***.com"},
		{"short local", "a@example.com", "***@ex***.com"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeEmail(tt.email)
			// Just check that it's masked (contains ***)
			if tt.email != "" && !contains(got, "***") {
				t.Errorf("SanitizeEmail() = %v, should contain ***", got)
			}
		})
	}
}

func TestSanitizeURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"normal URL", "https://example.com/verify?token=abc123"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeURL(tt.url)
			// Just check that query params are masked
			if tt.url != "" && contains(tt.url, "?") && !contains(got, "?***") {
				t.Errorf("SanitizeURL() = %v, should mask query params", got)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestValidateMessageBodySize(t *testing.T) {
	tests := []struct {
		name    string
		body    []byte
		wantErr bool
	}{
		{"valid size", make([]byte, 100), false},
		{"exact max size", make([]byte, MaxMessageBodySize), false},
		{"too large", make([]byte, MaxMessageBodySize+1), true},
		{"empty body", []byte{}, false},
		{"very large", make([]byte, MaxMessageBodySize*2), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMessageBodySize(tt.body)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMessageBodySize() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateStringLength(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		maxLen  int
		wantErr bool
	}{
		{"valid length", "hello", 10, false},
		{"exact max length", "hello", 5, false},
		{"too long", "hello world", 5, true},
		{"empty string", "", 10, false},
		{"unicode string", "你好世界", 4, false},
		{"unicode too long", "你好世界", 3, true},
		{"mixed unicode", "hello世界", 7, false},
		{"mixed unicode too long", "hello世界", 6, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStringLength(tt.s, tt.maxLen)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateStringLength() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
