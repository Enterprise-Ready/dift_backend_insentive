package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Service provides cryptographic operations
type Service struct {
	encryptionKey []byte
}

func NewService(encryptionKey string) (*Service, error) {
	key, err := base64.StdEncoding.DecodeString(encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("invalid encryption key: %w", err)
	}
	if len(key) != 32 {
		return nil, errors.New("encryption key must be 32 bytes (AES-256)")
	}
	return &Service{encryptionKey: key}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM
func (s *Service) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts ciphertext using AES-256-GCM
func (s *Service) Decrypt(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, cipherdata := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, cipherdata, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// HashAPIKey hashes an API key using bcrypt
func HashAPIKey(key string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyAPIKey verifies an API key against a bcrypt hash
func VerifyAPIKey(key, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(key))
	return err == nil
}

// HMACSHA256 creates HMAC-SHA256 signature
func HMACSHA256(message, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyHMAC verifies HMAC signature using constant-time comparison
func VerifyHMAC(message, secret, signature string) bool {
	expected := HMACSHA256(message, secret)
	return subtle.ConstantTimeCompare([]byte(expected), []byte(signature)) == 1
}

// SHA256Hash creates a one-way hash (for indexing sensitive data)
func SHA256Hash(data string) string {
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

// MaskCardNumber masks a card number, showing only last 4 digits
func MaskCardNumber(cardNumber string) string {
	// Remove spaces and dashes
	clean := regexp.MustCompile(`[\s\-]`).ReplaceAllString(cardNumber, "")
	if len(clean) < 4 {
		return "****"
	}
	last4 := clean[len(clean)-4:]
	masked := strings.Repeat("*", len(clean)-4) + last4
	// Re-format with spaces every 4 chars
	var formatted []string
	for i := 0; i < len(masked); i += 4 {
		end := i + 4
		if end > len(masked) {
			end = len(masked)
		}
		formatted = append(formatted, masked[i:end])
	}
	return strings.Join(formatted, " ")
}

// ValidateLuhn validates a card number using Luhn algorithm
func ValidateLuhn(cardNumber string) bool {
	clean := regexp.MustCompile(`[\s\-]`).ReplaceAllString(cardNumber, "")
	if len(clean) < 13 || len(clean) > 19 {
		return false
	}

	sum := 0
	nDigits := len(clean)
	parity := nDigits % 2

	for i := 0; i < nDigits; i++ {
		digit, err := strconv.Atoi(string(clean[i]))
		if err != nil {
			return false
		}
		if i%2 == parity {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
	}
	return sum%10 == 0
}

// GetCardBrand identifies the card brand from a card number
func GetCardBrand(cardNumber string) string {
	clean := regexp.MustCompile(`[\s\-]`).ReplaceAllString(cardNumber, "")
	patterns := map[string]*regexp.Regexp{
		"Visa":             regexp.MustCompile(`^4`),
		"Mastercard":       regexp.MustCompile(`^5[1-5]|^2[2-7]`),
		"American Express": regexp.MustCompile(`^3[47]`),
		"JCB":              regexp.MustCompile(`^35`),
		"UnionPay":         regexp.MustCompile(`^62`),
		"Discover":         regexp.MustCompile(`^6(?:011|5)`),
	}
	for brand, pattern := range patterns {
		if pattern.MatchString(clean) {
			return brand
		}
	}
	return "Unknown"
}

// ValidateCardExpiry validates card expiry in MM/YY or MM/YYYY format
func ValidateCardExpiry(expiry string) bool {
	parts := strings.Split(expiry, "/")
	if len(parts) != 2 {
		return false
	}
	month, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	year, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil {
		return false
	}
	if month < 1 || month > 12 {
		return false
	}
	// Handle 2-digit year
	if year < 100 {
		year += 2000
	}
	now := time.Now()
	expiryDate := time.Date(year, time.Month(month)+1, 0, 23, 59, 59, 0, time.UTC)
	return expiryDate.After(now)
}

// GenerateSecureToken generates a cryptographically secure random token
func GenerateSecureToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b)[:length], nil
}

// MaskEmail masks an email address for logs
func MaskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "****"
	}
	name := parts[0]
	if len(name) <= 2 {
		return "**@" + parts[1]
	}
	return name[:2] + strings.Repeat("*", len(name)-2) + "@" + parts[1]
}

// MaskPhone masks a phone number
func MaskPhone(phone string) string {
	if len(phone) < 4 {
		return "****"
	}
	return strings.Repeat("*", len(phone)-4) + phone[len(phone)-4:]
}
