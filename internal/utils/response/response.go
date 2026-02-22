// Package response provides helpers for writing consistent JSON HTTP responses.
//
// Every handler in this application sends JSON back to the client.
// Rather than repeating the same three lines (set header, set status,
// encode JSON) in every handler, we centralise them here.
//
// Consistent response shapes also make life easier for API consumers —
// they always know what error responses look like.
package response

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
)

// ─────────────────────────────────────────────────────────────────────────────
// Response is the standard envelope returned for error cases.
//
// Success responses may return any JSON shape (a student, a list, an id…).
// Error responses always look like:
//
//	{ "status": "error", "error": "field Name is required" }
//
// The json:"..." struct tags control the JSON key names.
// Without them Go would use capitalised field names ("Status", "Error").
// ─────────────────────────────────────────────────────────────────────────────
type Response struct {
	Status string `json:"status"` // "ok" or "error"
	Error  string `json:"error"`  // human-readable error detail
}

// Status string constants — use these instead of raw string literals so
// a typo is caught by the compiler rather than silently sending "eroor".
const (
	StatusOK    = "ok"
	StatusError = "error"
)

// ─────────────────────────────────────────────────────────────────────────────
// WriteJSON writes a JSON-encoded response with the given HTTP status code.
//
// Parameters:
//
//	w      — the http.ResponseWriter provided to every handler
//	status — HTTP status code (e.g. http.StatusOK = 200)
//	data   — any Go value; will be JSON-encoded and written to the body
//
// The "any" type (alias for interface{}) means data can be a struct, map,
// slice, or primitive — WriteJSON doesn't care.
//
// IMPORTANT ORDER: Header() → WriteHeader() → body writes.
// Once WriteHeader is called (or the first Write), headers are locked.
// ─────────────────────────────────────────────────────────────────────────────
func WriteJSON(w http.ResponseWriter, status int, data any) error {
	// Tell the client the body is JSON, not HTML or plain text.
	w.Header().Set("Content-Type", "application/json")

	// Write the HTTP status line (e.g. "HTTP/1.1 201 Created").
	// This must happen before any body bytes are written.
	w.WriteHeader(status)

	// json.NewEncoder(w) creates a JSON encoder that streams directly
	// into w, avoiding an intermediate buffer.
	// Encode() appends a newline after the JSON — handy for CLI testing.
	return json.NewEncoder(w).Encode(data)
}

// ─────────────────────────────────────────────────────────────────────────────
// GeneralError wraps any Go error into our standard Response shape.
// Use this for unexpected errors (DB failures, decode errors, etc.)
//
// Example usage:
//
//	response.WriteJSON(w, http.StatusInternalServerError,
//	    response.GeneralError(err))
//
// ─────────────────────────────────────────────────────────────────────────────
func GeneralError(err error) Response {
	return Response{
		Status: StatusError,
		Error:  err.Error(), // .Error() returns the error message string
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// ValidationError converts a slice of validator.FieldError values into
// a single human-readable Response.
//
// The go-playground/validator package returns one FieldError per failing
// struct field. We convert each to a plain English sentence and join them
// with ", " so the client sees a single descriptive error string.
//
// Example output:
//
//	{ "status": "error", "error": "field Name is required, field Age is required" }
//
// ─────────────────────────────────────────────────────────────────────────────
func ValidationError(errs validator.ValidationErrors) Response {
	var errMessages []string

	for _, e := range errs {
		switch e.ActualTag() {
		// "required" tag — field was missing or zero-valued
		case "required":
			errMessages = append(errMessages,
				fmt.Sprintf("field %s is required", e.Field()))
		// "email" tag — field did not match email format
		case "email":
			errMessages = append(errMessages,
				fmt.Sprintf("field %s must be a valid email address", e.Field()))
		// Catch-all for any other validation tag (min, max, len, etc.)
		default:
			errMessages = append(errMessages,
				fmt.Sprintf("field %s is invalid", e.Field()))
		}
	}

	return Response{
		Status: StatusError,
		// strings.Join(slice, sep) concatenates a slice of strings
		// with the given separator between each element.
		Error: strings.Join(errMessages, ", "),
	}
}
