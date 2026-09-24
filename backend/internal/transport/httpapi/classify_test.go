package httpapi

import (
	"errors"
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v3"

	"dommax/internal/app"
	"dommax/internal/domain/issue"
)

func TestClassifyMapsEveryErrorKind(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{app.ErrUnauthorized, 401, "unauthorized"},
		{app.ErrConsentRequired, 403, "consent_required"},
		{app.ErrForbidden, 403, "forbidden"},
		{app.ErrNotFound, 404, "not_found"},
		{issue.ErrAlreadyJoined, 409, "already_joined"},
		{issue.ErrClosed, 409, "issue_closed"},
		{fmt.Errorf("wrapped: %w", issue.ErrTransition), 409, "invalid_transition"},
		{issue.ErrReasonRequired, 422, "reason_required"},
		{app.ErrInvalidInput, 422, "invalid_input"},
		{issue.ErrInvalid, 422, "invalid_input"},
		{fiber.ErrMethodNotAllowed, 405, "http_error"},
		{errors.New("database is down"), 500, "internal"},
	}
	for _, tc := range cases {
		status, code, msg := classify(tc.err)
		if status != tc.status || code != tc.code || msg == "" {
			t.Errorf("classify(%v) = %d %q %q, want %d %q", tc.err, status, code, msg, tc.status, tc.code)
		}
	}
}

func TestInternalErrorDoesNotLeakDetails(t *testing.T) {
	_, _, msg := classify(errors.New("pq: password authentication failed for user dommax"))
	if msg != "Что-то пошло не так. Попробуйте позже" {
		t.Fatalf("message leaks internals: %q", msg)
	}
}
