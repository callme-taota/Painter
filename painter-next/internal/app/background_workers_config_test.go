package app

import (
	"bytes"
	"encoding/base64"
	"os"
	"strings"
	"testing"
)

func backgroundConfigEnv(t *testing.T) {
	t.Helper()
	// Configuration parsing must not depend on a live database or operator keys.
	t.Setenv("DATABASE_URL", "host=/nonexistent dbname=config_test")
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("PUBLIC_BASE_URL", "https://painter.test")
	t.Setenv("APP_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)))
	t.Setenv("ANALYTICS_HMAC_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{2}, 32)))
	for _, key := range []string{"GITHUB_CLIENT_ID", "GITHUB_CLIENT_SECRET", "GITHUB_CALLBACK_URL", "BOOTSTRAP_GITHUB_ID", "SETUP_TOKEN"} {
		t.Setenv(key, "")
	}
}

func TestBackgroundWorkersConfigDefaultsAndExplicitValues(t *testing.T) {
	backgroundConfigEnv(t)
	t.Setenv("RUN_BACKGROUND_WORKERS", "")
	if err := os.Unsetenv("RUN_BACKGROUND_WORKERS"); err != nil {
		t.Fatal(err)
	}
	c, err := ConfigFromEnv()
	if err != nil || !c.BackgroundWorkersEnabled() || c.RunBackgroundWorkers != nil {
		t.Fatal("absent setting must retain existing workers", err)
	}
	if !(Config{}).BackgroundWorkersEnabled() {
		t.Fatal("direct Config callers lost default workers")
	}
	for _, sample := range []struct {
		raw  string
		want bool
	}{{"true", true}, {"false", false}, {"TRUE", true}, {"0", false}, {"1", true}} {
		t.Run(sample.raw, func(t *testing.T) {
			t.Setenv("RUN_BACKGROUND_WORKERS", sample.raw)
			c, err := ConfigFromEnv()
			if err != nil || c.RunBackgroundWorkers == nil || c.BackgroundWorkersEnabled() != sample.want {
				t.Fatal("explicit worker configuration not preserved", sample.raw, err)
			}
		})
	}
}

func TestBackgroundWorkersConfigRejectsInvalidValues(t *testing.T) {
	backgroundConfigEnv(t)
	for _, raw := range []string{"", "yes", "no", "2", " false ", "disabled", "invalid\nvalue"} {
		t.Run(raw, func(t *testing.T) {
			t.Setenv("RUN_BACKGROUND_WORKERS", raw)
			_, err := ConfigFromEnv()
			if err == nil || !strings.Contains(err.Error(), "RUN_BACKGROUND_WORKERS") {
				t.Fatal("invalid setting should reject startup", err)
			}
			if strings.Contains(err.Error(), raw) && raw != "" && raw != " false " {
				t.Fatal("configuration error should not echo the environment value")
			}
		})
	}
}
