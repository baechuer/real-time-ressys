package main

import (
	"reflect"
	"testing"

	"github.com/baechuer/real-time-ressys/services/auth-service/app/dto"
	"github.com/go-playground/validator/v10"
	ut "github.com/go-playground/universal-translator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
Validation Test Cases:

1. TestValidatePasswordStrength_Valid
   - Password with uppercase, lowercase, and number
   - Returns true

2. TestValidatePasswordStrength_MissingUppercase
   - Password missing uppercase letter
   - Returns false

3. TestValidatePasswordStrength_MissingLowercase
   - Password missing lowercase letter
   - Returns false

4. TestValidatePasswordStrength_MissingNumber
   - Password missing number
   - Returns false

5. TestValidatePasswordStrength_EmptyPassword
   - Empty password
   - Returns false

6. TestValidateUsernameFormat_Valid
   - Username with letters, numbers, and underscores
   - Returns true

7. TestValidateUsernameFormat_InvalidCharacters
   - Username with special characters
   - Returns false

8. TestValidateUsernameFormat_EmptyUsername
   - Empty username
   - Returns false

9. TestValidateRequest_Valid
   - Valid RegisterRequest
   - Returns nil (no error)

10. TestValidateRequest_Invalid
    - Invalid RegisterRequest (missing fields, invalid format)
    - Returns AppError with validation messages

11. TestFormatFieldError_AllTags
    - Tests all error tag formats (required, email, min, max, password_strength, username_format)
    - Returns formatted error messages

12. TestSanitizeInput_Trimming
    - Input with leading/trailing whitespace
    - Whitespace trimmed

13. TestSanitizeInput_ControlCharacters
    - Input with control characters
    - Control characters removed

14. TestSanitizeInput_MaxLength
    - Input exceeding max length
    - Input truncated to max length

15. TestSanitizeInput_PreserveSpecialChars
    - Password input with special characters
    - Special characters preserved

16. TestSanitizeEmail_Lowercase
    - Email with uppercase letters
    - Converted to lowercase

17. TestSanitizeEmail_Trimming
    - Email with whitespace
    - Whitespace trimmed

18. TestSanitizeUsername_RemoveSpecialChars
    - Username with special characters
    - Special characters removed

19. TestSanitizeUsername_KeepValidChars
    - Username with letters, numbers, underscores
    - Valid characters kept
*/

// TestValidatePasswordStrength_Valid tests password with all requirements
func TestValidatePasswordStrength_Valid(t *testing.T) {
	testCases := []string{
		"Password123",
		"Test123",
		"MyPass1",
		"ABCdef123",
		"P@ssw0rd", // Special chars allowed
	}

	for _, password := range testCases {
		t.Run(password, func(t *testing.T) {
			result := testPasswordStrength(t, password)
			assert.True(t, result, "Password '%s' should be valid", password)
		})
	}
}

// TestValidatePasswordStrength_MissingUppercase tests password missing uppercase
func TestValidatePasswordStrength_MissingUppercase(t *testing.T) {
	testCases := []string{
		"password123",
		"test123",
		"mypass1",
	}

	for _, password := range testCases {
		t.Run(password, func(t *testing.T) {
			result := testPasswordStrength(t, password)
			assert.False(t, result, "Password '%s' should be invalid (missing uppercase)", password)
		})
	}
}

// TestValidatePasswordStrength_MissingLowercase tests password missing lowercase
func TestValidatePasswordStrength_MissingLowercase(t *testing.T) {
	testCases := []string{
		"PASSWORD123",
		"TEST123",
		"MYPASS1",
	}

	for _, password := range testCases {
		t.Run(password, func(t *testing.T) {
			result := testPasswordStrength(t, password)
			assert.False(t, result, "Password '%s' should be invalid (missing lowercase)", password)
		})
	}
}

// TestValidatePasswordStrength_MissingNumber tests password missing number
func TestValidatePasswordStrength_MissingNumber(t *testing.T) {
	testCases := []string{
		"Password",
		"Test",
		"MyPass",
	}

	for _, password := range testCases {
		t.Run(password, func(t *testing.T) {
			result := testPasswordStrength(t, password)
			assert.False(t, result, "Password '%s' should be invalid (missing number)", password)
		})
	}
}

// TestValidatePasswordStrength_EmptyPassword tests empty password
func TestValidatePasswordStrength_EmptyPassword(t *testing.T) {
	result := testPasswordStrength(t, "")
	assert.False(t, result, "Empty password should be invalid")
}

// TestValidateUsernameFormat_Valid tests valid username formats
func TestValidateUsernameFormat_Valid(t *testing.T) {
	testCases := []string{
		"testuser",
		"test_user",
		"test123",
		"user_123",
		"TestUser",
		"USER123",
		"_username",
		"user_name_123",
	}

	for _, username := range testCases {
		t.Run(username, func(t *testing.T) {
			result := testUsernameFormat(t, username)
			assert.True(t, result, "Username '%s' should be valid", username)
		})
	}
}

// TestValidateUsernameFormat_InvalidCharacters tests username with invalid characters
func TestValidateUsernameFormat_InvalidCharacters(t *testing.T) {
	testCases := []string{
		"user@name",
		"user-name",
		"user.name",
		"user name",
		"user!name",
		"user#name",
		"user$name",
	}

	for _, username := range testCases {
		t.Run(username, func(t *testing.T) {
			result := testUsernameFormat(t, username)
			assert.False(t, result, "Username '%s' should be invalid", username)
		})
	}
}

// TestValidateUsernameFormat_EmptyUsername tests empty username
func TestValidateUsernameFormat_EmptyUsername(t *testing.T) {
	result := testUsernameFormat(t, "")
	assert.False(t, result, "Empty username should be invalid")
}

// TestValidateRequest_Valid tests valid request
func TestValidateRequest_Valid(t *testing.T) {
	req := dto.RegisterRequest{
		Email:    "test@example.com",
		Username: "testuser",
		Password: "Password123",
	}

	err := validateRequest(&req)
	assert.Nil(t, err, "Valid request should not return error")
}

// TestValidateRequest_Invalid tests invalid request
func TestValidateRequest_Invalid(t *testing.T) {
	req := dto.RegisterRequest{
		Email:    "invalid-email",
		Username: "ab",
		Password: "pass",
	}

	err := validateRequest(&req)
	require.NotNil(t, err, "Invalid request should return error")
	assert.Equal(t, "INVALID_INPUT", string(err.Code))
	assert.Contains(t, err.Message, "Email")
	assert.Contains(t, err.Message, "Username")
	assert.Contains(t, err.Message, "Password")
}

// TestFormatFieldError_AllTags tests all error tag formats
func TestFormatFieldError_AllTags(t *testing.T) {
	testCases := []struct {
		tag      string
		field    string
		param    string
		expected string
	}{
		{"required", "Email", "", "Email is required"},
		{"email", "Email", "", "Email must be a valid email address"},
		{"min", "Password", "8", "Password must be at least 8 characters"},
		{"max", "Username", "50", "Username must be at most 50 characters"},
		{"password_strength", "Password", "", "Password must contain at least one uppercase letter, one lowercase letter, and one number"},
		{"username_format", "Username", "", "Username can only contain letters, numbers, and underscores"},
		{"unknown", "Field", "", "Field is invalid"},
	}

	for _, tc := range testCases {
		t.Run(tc.tag, func(t *testing.T) {
			fe := &mockFieldError{
				field: tc.field,
				tag:   tc.tag,
				param: tc.param,
			}
			result := formatFieldError(fe)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestSanitizeInput_Trimming tests whitespace trimming
func TestSanitizeInput_Trimming(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"  test  ", "test"},
		{"\ttest\n", "test"},
		{"  test@example.com  ", "test@example.com"},
		{"   ", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := sanitizeInput(tc.input, 0, false)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestSanitizeInput_ControlCharacters tests control character removal
func TestSanitizeInput_ControlCharacters(t *testing.T) {
	input := "test\x00\x01\x02string"
	result := sanitizeInput(input, 0, false)
	assert.Equal(t, "teststring", result)
	assert.NotContains(t, result, "\x00")
}

// TestSanitizeInput_MaxLength tests max length truncation
func TestSanitizeInput_MaxLength(t *testing.T) {
	input := "this is a very long string"
	maxLength := 10
	result := sanitizeInput(input, maxLength, false)
	
	// Should be truncated to 10 characters
	assert.LessOrEqual(t, len([]rune(result)), maxLength)
	assert.Equal(t, "this is a ", result)
}

// TestSanitizeInput_PreserveSpecialChars tests password special character preservation
func TestSanitizeInput_PreserveSpecialChars(t *testing.T) {
	input := "  P@ssw0rd!  "
	result := sanitizeInput(input, 0, true)
	
	// Should preserve special characters, only trim
	assert.Equal(t, "P@ssw0rd!", result)
}

// TestSanitizeInput_PreserveSpecialChars_MaxLength tests password with max length
func TestSanitizeInput_PreserveSpecialChars_MaxLength(t *testing.T) {
	input := "VeryLongPassword123!@#"
	maxLength := 15
	result := sanitizeInput(input, maxLength, true)
	
	// Should truncate to max length (special chars may be cut off if at the end)
	assert.LessOrEqual(t, len([]rune(result)), maxLength)
	// Verify it's truncated correctly
	assert.Equal(t, maxLength, len([]rune(result)))
}

// TestSanitizeEmail_Lowercase tests email lowercase conversion
func TestSanitizeEmail_Lowercase(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"Test@Example.COM", "test@example.com"},
		{"USER@DOMAIN.COM", "user@domain.com"},
		{"Test.User@Example.COM", "test.user@example.com"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := sanitizeEmail(tc.input, 255)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestSanitizeEmail_Trimming tests email whitespace trimming
func TestSanitizeEmail_Trimming(t *testing.T) {
	input := "  Test@Example.COM  "
	result := sanitizeEmail(input, 255)
	assert.Equal(t, "test@example.com", result)
}

// TestSanitizeEmail_MaxLength tests email max length
func TestSanitizeEmail_MaxLength(t *testing.T) {
	input := "verylongemailaddressthatshouldbetruncated@example.com"
	maxLength := 30
	result := sanitizeEmail(input, maxLength)
	assert.LessOrEqual(t, len([]rune(result)), maxLength)
}

// TestSanitizeUsername_RemoveSpecialChars tests special character removal
func TestSanitizeUsername_RemoveSpecialChars(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"user@name", "username"},
		{"user-name", "username"},
		{"user.name", "username"},
		{"user!name", "username"},
		{"user#name", "username"},
		{"user name", "username"},
		{"user_name", "user_name"}, // Underscore should be kept
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := sanitizeUsername(tc.input, 50)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestSanitizeUsername_KeepValidChars tests valid character preservation
func TestSanitizeUsername_KeepValidChars(t *testing.T) {
	testCases := []string{
		"testuser",
		"test_user",
		"test123",
		"user_123",
		"TestUser",
		"USER123",
	}

	for _, username := range testCases {
		t.Run(username, func(t *testing.T) {
			result := sanitizeUsername(username, 50)
			assert.Equal(t, username, result, "Valid characters should be preserved")
		})
	}
}

// TestSanitizeUsername_Trimming tests username whitespace trimming
func TestSanitizeUsername_Trimming(t *testing.T) {
	input := "  testuser  "
	result := sanitizeUsername(input, 50)
	assert.Equal(t, "testuser", result)
}

// TestSanitizeUsername_MaxLength tests username max length
func TestSanitizeUsername_MaxLength(t *testing.T) {
	input := "verylongusernamethatshouldbetruncated"
	maxLength := 10
	result := sanitizeUsername(input, maxLength)
	assert.LessOrEqual(t, len([]rune(result)), maxLength)
}

// Helper function to test password strength validator
func testPasswordStrength(t *testing.T, password string) bool {
	type testStruct struct {
		Password string `validate:"password_strength"`
	}

	ts := testStruct{Password: password}
	testValidate := validator.New()
	testValidate.RegisterValidation("password_strength", validatePasswordStrength)

	err := testValidate.Struct(ts)
	return err == nil
}

// Helper function to test username format validator
func testUsernameFormat(t *testing.T, username string) bool {
	type testStruct struct {
		Username string `validate:"username_format"`
	}

	ts := testStruct{Username: username}
	testValidate := validator.New()
	testValidate.RegisterValidation("username_format", validateUsernameFormat)

	err := testValidate.Struct(ts)
	return err == nil
}

// mockFieldError implements validator.FieldError for testing
type mockFieldError struct {
	field string
	tag   string
	param string
}

func (m *mockFieldError) Tag() string {
	return m.tag
}

func (m *mockFieldError) ActualTag() string {
	return m.tag
}

func (m *mockFieldError) Namespace() string {
	return ""
}

func (m *mockFieldError) StructNamespace() string {
	return ""
}

func (m *mockFieldError) Field() string {
	return m.field
}

func (m *mockFieldError) StructField() string {
	return m.field
}

func (m *mockFieldError) Value() interface{} {
	return ""
}

func (m *mockFieldError) Param() string {
	return m.param
}

func (m *mockFieldError) Kind() reflect.Kind {
	return reflect.String
}

func (m *mockFieldError) Type() reflect.Type {
	return reflect.TypeOf("")
}

func (m *mockFieldError) Translate(ut ut.Translator) string {
	return ""
}

func (m *mockFieldError) Error() string {
	return ""
}

