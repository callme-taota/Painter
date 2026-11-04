package app

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// OWASP Password Storage Cheat Sheet: Argon2id >=19 MiB, t=2, p=1.
// Two concurrent derivations bound this process to 38 MiB of Argon2 memory.
const passwordMemory uint32 = 19 * 1024
const passwordIterations uint32 = 2
const passwordParallelism uint8 = 1
const passwordMinLength = 15
const passwordMaxLength = 1024
const passwordMaxBytes = 4096

var passwordSlots = make(chan struct{}, 2)

// This is deliberately not a credential. Unknown usernames perform the same
// derivation against a fixed, unmatchable placeholder instead of skipping Argon2.
const dummyPasswordHash = "$argon2id$v=19$m=19456,t=2,p=1$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

func validatePassword(password string) error {
	if !utf8.ValidString(password) || utf8.RuneCountInString(password) < passwordMinLength || utf8.RuneCountInString(password) > passwordMaxLength || len(password) > passwordMaxBytes || strings.ContainsRune(password, '\x00') {
		return E(400, "VALIDATION_FAILED", "密码须为15至1024个字符，最多4096字节；允许空格和长口令")
	}
	return nil
}
func withPasswordSlot(ctx context.Context, fn func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case passwordSlots <- struct{}{}:
		defer func() { <-passwordSlots }()
		return fn()
	default:
		return E(503, "AUTH_BUSY", "登录服务繁忙，请稍后重试")
	}
}
func hashPassword(ctx context.Context, password string) (string, error) {
	if err := validatePassword(password); err != nil {
		return "", err
	}
	var encoded string
	err := withPasswordSlot(ctx, func() error {
		salt := make([]byte, 16)
		if _, err := rand.Read(salt); err != nil {
			return err
		}
		sum := argon2.IDKey([]byte(password), salt, passwordIterations, passwordMemory, passwordParallelism, 32)
		encoded = fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", passwordMemory, passwordIterations, passwordParallelism, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(sum))
		return nil
	})
	return encoded, err
}
func verifyPassword(ctx context.Context, password, encoded string) (bool, error) {
	// Reject unreasonable inputs without allocating Argon2 memory. Never truncate.
	if !utf8.ValidString(password) || len(password) > passwordMaxBytes {
		return false, nil
	}
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" || parts[3] != "m=19456,t=2,p=1" {
		return false, fmt.Errorf("unsupported password hash format")
	}
	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil || len(salt) != 16 {
		return false, fmt.Errorf("invalid password salt")
	}
	expected, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil || len(expected) != 32 {
		return false, fmt.Errorf("invalid password digest")
	}
	var matched bool
	err = withPasswordSlot(ctx, func() error {
		got := argon2.IDKey([]byte(password), salt, passwordIterations, passwordMemory, passwordParallelism, 32)
		matched = subtle.ConstantTimeCompare(got, expected) == 1
		return nil
	})
	return matched, err
}
