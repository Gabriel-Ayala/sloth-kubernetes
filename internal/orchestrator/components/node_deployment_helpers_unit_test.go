package components

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGenerateSecurePassword_Length tests that generated passwords have correct length
func TestGenerateSecurePassword_Length(t *testing.T) {
	password := generateSecurePassword()
	assert.Equal(t, 16, len(password), "Password should be exactly 16 characters long")
}

// TestGenerateSecurePassword_Charset tests that password contains only valid characters
func TestGenerateSecurePassword_Charset(t *testing.T) {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()"
	password := generateSecurePassword()

	for _, char := range password {
		assert.Contains(t, charset, string(char), "Password should only contain characters from the charset")
	}
}

// TestGenerateSecurePassword_NotEmpty tests that password is never empty
func TestGenerateSecurePassword_NotEmpty(t *testing.T) {
	password := generateSecurePassword()
	assert.NotEmpty(t, password, "Password should not be empty")
}

// TestGenerateSecurePassword_Uniqueness tests that multiple calls generate different passwords
func TestGenerateSecurePassword_Uniqueness(t *testing.T) {
	// Generate multiple passwords
	passwords := make(map[string]bool)
	iterations := 100

	for i := 0; i < iterations; i++ {
		password := generateSecurePassword()
		passwords[password] = true
	}

	// All passwords should be unique (extremely high probability with crypto/rand)
	assert.GreaterOrEqual(t, len(passwords), 99, "At least 99 out of 100 passwords should be unique")
}

// TestGenerateSecurePassword_MultipleGeneration tests consistency across multiple generations
func TestGenerateSecurePassword_MultipleGeneration(t *testing.T) {
	for i := 0; i < 10; i++ {
		password := generateSecurePassword()
		assert.Equal(t, 16, len(password), "All generated passwords should be 16 characters")
		assert.NotEmpty(t, password, "Generated password should not be empty")
	}
}

// TestGenerateSecurePassword_HasLowercase tests that password typically contains lowercase letters
func TestGenerateSecurePassword_HasLowercase(t *testing.T) {
	// Generate several passwords and check if at least some contain lowercase
	foundLowercase := false
	for i := 0; i < 20; i++ {
		password := generateSecurePassword()
		for _, char := range password {
			if char >= 'a' && char <= 'z' {
				foundLowercase = true
				break
			}
		}
		if foundLowercase {
			break
		}
	}
	assert.True(t, foundLowercase, "At least one password out of 20 should contain lowercase letters")
}

// TestGenerateSecurePassword_HasUppercase tests that password typically contains uppercase letters
func TestGenerateSecurePassword_HasUppercase(t *testing.T) {
	// Generate several passwords and check if at least some contain uppercase
	foundUppercase := false
	for i := 0; i < 20; i++ {
		password := generateSecurePassword()
		for _, char := range password {
			if char >= 'A' && char <= 'Z' {
				foundUppercase = true
				break
			}
		}
		if foundUppercase {
			break
		}
	}
	assert.True(t, foundUppercase, "At least one password out of 20 should contain uppercase letters")
}

// TestGenerateSecurePassword_HasDigits tests that password typically contains digits
func TestGenerateSecurePassword_HasDigits(t *testing.T) {
	// Generate several passwords and check if at least some contain digits
	foundDigit := false
	for i := 0; i < 20; i++ {
		password := generateSecurePassword()
		for _, char := range password {
			if char >= '0' && char <= '9' {
				foundDigit = true
				break
			}
		}
		if foundDigit {
			break
		}
	}
	assert.True(t, foundDigit, "At least one password out of 20 should contain digits")
}

// TestGenerateSecurePassword_HasSpecialChars tests that password typically contains special characters
func TestGenerateSecurePassword_HasSpecialChars(t *testing.T) {
	specialChars := "!@#$%^&*()"
	// Generate several passwords and check if at least some contain special chars
	foundSpecial := false
	for i := 0; i < 20; i++ {
		password := generateSecurePassword()
		for _, char := range password {
			if strings.ContainsRune(specialChars, char) {
				foundSpecial = true
				break
			}
		}
		if foundSpecial {
			break
		}
	}
	assert.True(t, foundSpecial, "At least one password out of 20 should contain special characters")
}

// TestGenerateSecurePassword_NoWhitespace tests that password never contains whitespace
func TestGenerateSecurePassword_NoWhitespace(t *testing.T) {
	for i := 0; i < 50; i++ {
		password := generateSecurePassword()
		assert.NotContains(t, password, " ", "Password should not contain spaces")
		assert.NotContains(t, password, "\t", "Password should not contain tabs")
		assert.NotContains(t, password, "\n", "Password should not contain newlines")
	}
}

// TestGenerateSecurePassword_CharacterDistribution tests character distribution
func TestGenerateSecurePassword_CharacterDistribution(t *testing.T) {
	// Generate many passwords and verify reasonable character distribution
	charCount := make(map[rune]int)
	iterations := 1000

	for i := 0; i < iterations; i++ {
		password := generateSecurePassword()
		for _, char := range password {
			charCount[char]++
		}
	}

	// Should have seen many different characters
	assert.Greater(t, len(charCount), 30, "Should use a variety of characters across 1000 passwords")
}

// TestGenerateSecurePassword_NoControlCharacters tests that password has no control characters
func TestGenerateSecurePassword_NoControlCharacters(t *testing.T) {
	for i := 0; i < 50; i++ {
		password := generateSecurePassword()
		for _, char := range password {
			assert.False(t, char < 32 || char == 127, "Password should not contain control characters")
		}
	}
}

// TestGenerateSecurePassword_Determinism tests that function is called correctly
func TestGenerateSecurePassword_Determinism(t *testing.T) {
	// Just verify function can be called multiple times without panic
	for i := 0; i < 100; i++ {
		password := generateSecurePassword()
		assert.NotNil(t, password)
		assert.Equal(t, 16, len(password))
	}
}

// TestGenerateSecurePassword_StringType tests that return type is string
func TestGenerateSecurePassword_StringType(t *testing.T) {
	password := generateSecurePassword()
	assert.IsType(t, "", password, "Password should be of type string")
}

// TestGenerateSecurePassword_AllCharacterTypes tests for presence of all character types
func TestGenerateSecurePassword_AllCharacterTypes(t *testing.T) {
	// Generate many passwords and verify we eventually see all types
	hasLowercase := false
	hasUppercase := false
	hasDigit := false
	hasSpecial := false
	specialChars := "!@#$%^&*()"

	for i := 0; i < 100; i++ {
		password := generateSecurePassword()

		for _, char := range password {
			if char >= 'a' && char <= 'z' {
				hasLowercase = true
			}
			if char >= 'A' && char <= 'Z' {
				hasUppercase = true
			}
			if char >= '0' && char <= '9' {
				hasDigit = true
			}
			if strings.ContainsRune(specialChars, char) {
				hasSpecial = true
			}
		}

		// If we've seen all types, we can stop early
		if hasLowercase && hasUppercase && hasDigit && hasSpecial {
			break
		}
	}

	assert.True(t, hasLowercase, "Should generate passwords with lowercase letters")
	assert.True(t, hasUppercase, "Should generate passwords with uppercase letters")
	assert.True(t, hasDigit, "Should generate passwords with digits")
	assert.True(t, hasSpecial, "Should generate passwords with special characters")
}

// TestGenerateSecurePassword_EntropyCheck tests for sufficient entropy
func TestGenerateSecurePassword_EntropyCheck(t *testing.T) {
	passwords := make(map[string]bool)
	duplicateCount := 0
	iterations := 1000

	for i := 0; i < iterations; i++ {
		password := generateSecurePassword()
		if passwords[password] {
			duplicateCount++
		}
		passwords[password] = true
	}

	// With crypto/rand, duplicates should be virtually impossible
	assert.Less(t, duplicateCount, 5, "Should have very few (< 5) duplicate passwords in 1000 generations")
	assert.GreaterOrEqual(t, len(passwords), 995, "Should have at least 995 unique passwords out of 1000")
}

// TestGenerateSecurePassword_NoObviousPatterns tests that password doesn't have obvious patterns
func TestGenerateSecurePassword_NoObviousPatterns(t *testing.T) {
	for i := 0; i < 50; i++ {
		password := generateSecurePassword()

		// Password should not be all same character
		allSame := true
		firstChar := rune(password[0])
		for _, char := range password {
			if char != firstChar {
				allSame = false
				break
			}
		}
		assert.False(t, allSame, "Password should not be all the same character")

		// Password should not be sequential (like "0123456789abcdef")
		sequential := true
		for j := 0; j < len(password)-1; j++ {
			if password[j+1] != password[j]+1 {
				sequential = false
				break
			}
		}
		assert.False(t, sequential, "Password should not be sequential")
	}
}
