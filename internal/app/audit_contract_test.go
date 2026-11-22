package app

import (
	"context"
	"errors"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Capture the existing transaction boundary: audit helpers must neither open a
// second connection nor commit independently of the business command.
type auditCaptureTx struct {
	pgx.Tx
	ctx  context.Context
	args []any
	err  error
}

func (tx *auditCaptureTx) Exec(ctx context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
	tx.ctx, tx.args = ctx, args
	return pgconn.CommandTag{}, tx.err
}

func TestAuditHelpersPreserveCommandIdentity(t *testing.T) {
	ctx := context.WithValue(context.Background(), requestIDKey, "synthetic-command-request")
	r := httptest.NewRequest("POST", "/api/v1/series", nil).WithContext(ctx)
	p := Principal{ID: "7", Role: "owner"}
	tests := []struct {
		name string
		call func(pgx.Tx) error
		want []any
	}{
		{"post", func(tx pgx.Tx) error { return contentAudit(ctx, tx, p, "post.save", "12") }, []any{"7", "post.save", "post", "12", "synthetic-command-request", ""}},
		{"interaction", func(tx pgx.Tx) error {
			return interactionAudit(tx, r, p, "series.update", "series", "15", "修改资料")
		}, []any{"7", "series.update", "series", "15", "synthetic-command-request", "修改资料"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := &auditCaptureTx{}
			before := NowMillis()
			if err := tt.call(tx); err != nil {
				t.Fatal(err)
			}
			if tx.ctx != ctx || len(tx.args) != 7 || !reflect.DeepEqual(tx.args[:6], tt.want) {
				t.Fatalf("audit lost command identity: %#v", tx.args)
			}
			stamp, ok := tx.args[6].(int64)
			if !ok || stamp < before || stamp > NowMillis() {
				t.Fatal("audit timestamp is outside command execution")
			}
			failure := errors.New("audit storage failed")
			tx.err = failure
			if !errors.Is(tt.call(tx), failure) {
				t.Fatal("audit failure must propagate to the caller's transaction")
			}
		})
	}
}

func TestInteractionAuditReasonLimitUnchanged(t *testing.T) {
	tx := &auditCaptureTx{}
	r := httptest.NewRequest("PUT", "/api/v1/admin/comments/1/visibility", nil)
	p := Principal{ID: "7"}
	if err := interactionAudit(tx, r, p, "comment.visibility", "comment", "1", strings.Repeat("字", 500)); err != nil {
		t.Fatal(err)
	}
	tx.args = nil
	err := interactionAudit(tx, r, p, "comment.visibility", "comment", "1", strings.Repeat("字", 501))
	var apiError *APIError
	if !errors.As(err, &apiError) || apiError.Status != 400 || apiError.Code != "VALIDATION_FAILED" || tx.args != nil {
		t.Fatalf("oversize reason must fail before storage: %v", err)
	}
}
