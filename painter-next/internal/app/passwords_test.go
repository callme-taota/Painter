package app

import (
	"context"
	"strings"
	"testing"
)

func TestPasswordHashSaltLongPassphraseAndNoTruncation(t *testing.T) {
	password := "  " + strings.Repeat("中文🌿 long passphrase ", 30) + "  "
	first, err := hashPassword(context.Background(), password)
	if err != nil {
		t.Fatal(err)
	}
	second, err := hashPassword(context.Background(), password)
	if err != nil || first == second {
		t.Fatal("missing unique salt", err)
	}
	if ok, err := verifyPassword(context.Background(), password, first); err != nil || !ok {
		t.Fatal("long Unicode phrase rejected", ok, err)
	}
	if ok, err := verifyPassword(context.Background(), strings.TrimSpace(password), first); err != nil || ok {
		t.Fatal("password was trimmed", ok, err)
	}
	if ok, err := verifyPassword(context.Background(), password+"x", first); err != nil || ok {
		t.Fatal("password silently truncated", ok, err)
	}
	for _, value := range []string{"short", strings.Repeat("x", 1025), strings.Repeat("🌿", 1025), "password-with-\x00-inside"} {
		if validatePassword(value) == nil {
			t.Fatal("invalid password accepted")
		}
	}
	if ok, err := verifyPassword(context.Background(), "anything", dummyPasswordHash); err != nil || ok {
		t.Fatal("invalid dummy hash", ok, err)
	}
	for _, hash := range []string{"plaintext", "$argon2id$v=19$m=999999999,t=2,p=1$AA$AA", strings.Replace(first, "v=19", "v=16", 1)} {
		if _, err := verifyPassword(context.Background(), password, hash); err == nil {
			t.Fatal("unsafe PHC parameters accepted")
		}
	}
}
func TestPasswordConcurrencyBound(t *testing.T) {
	passwordSlots <- struct{}{}
	passwordSlots <- struct{}{}
	defer func() { <-passwordSlots; <-passwordSlots }()
	if _, err := hashPassword(context.Background(), "correct horse battery staple"); err == nil {
		t.Fatal("unbounded concurrent hash")
	}
}
