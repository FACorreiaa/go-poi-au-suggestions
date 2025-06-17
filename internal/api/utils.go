package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5/middleware" // For RequestID
	"github.com/golang-jwt/jwt/v5"
)

// ErrorResponse writes a standard JSON error response including request ID.
func ErrorResponse(w http.ResponseWriter, r *http.Request, status int, message string) {
	reqID := middleware.GetReqID(r.Context()) // Get request ID if available
	resp := map[string]interface{}{           // Use interface{} for potential flexibility
		"success":    false,
		"error":      message,
		"request_id": reqID,
	}
	WriteJSONResponse(w, r, status, resp) // Call the common JSON writer
}

// WriteJSONResponse encodes the data to JSON and writes the response header and body.
func WriteJSONResponse(w http.ResponseWriter, r *http.Request, status int, data interface{}) {
	// If data is nil and status indicates no content, just write header
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}

	// Marshal payload
	js, err := json.Marshal(data)
	if err != nil {
		// Log the internal error
		reqID := middleware.GetReqID(r.Context())
		slog.ErrorContext(r.Context(), "Failed to marshal JSON response",
			slog.Any("error", err),
			slog.String("request_id", reqID),
		)
		// Send a generic server error response to the client
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Set headers *before* writing status or body
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status) // Write status code
	_, err = w.Write(js)  // Write JSON body
	if err != nil {
		// Log write error, client already received status code
		reqID := middleware.GetReqID(r.Context())
		slog.ErrorContext(r.Context(), "Failed to write response body",
			slog.Any("error", err),
			slog.String("request_id", reqID),
		)
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush() // Ensure data is sent immediately
	}
}

// DecodeJSONBody reads and decodes a JSON request body safely.
func DecodeJSONBody(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	// Set a max body size to prevent abuse (e.g., 1MB)
	maxBytes := 1_048_576
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes)) // Use ResponseWriter for MaxBytesReader

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(dst)
	if err != nil {
		// Handle various JSON decoding errors gracefully
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError
		var maxBytesError *http.MaxBytesError // Check for max bytes error

		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxError.Offset)

		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly-formed JSON")

		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field %q (wanted %s)", unmarshalTypeError.Field, unmarshalTypeError.Type)
			}
			return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)

		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")

		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			// Remove surrounding quotes if present
			fieldName = strings.Trim(fieldName, `"`)
			return fmt.Errorf("body contains unknown key %q", fieldName)

		// Check for MaxBytesError explicitly
		case errors.As(err, &maxBytesError):
			return fmt.Errorf("body must not be larger than %d bytes", maxBytesError.Limit)

		case errors.As(err, &invalidUnmarshalError):
			// This usually indicates a programming error (passing non-pointer)
			// Panic might be appropriate here during development
			panic(fmt.Errorf("developer error: invalid argument passed to json.Unmarshal: %w", err))

		default:
			return fmt.Errorf("error decoding JSON body: %w", err)
		}
	}

	// Check for trailing data after the first JSON object
	err = dec.Decode(&struct{}{})
	if !errors.Is(err, io.EOF) {
		return errors.New("body must only contain a single JSON value")
	}

	return nil
}

func VerifyAudience(claimsAudience jwt.ClaimStrings, expectedAudience string) bool {
	// If no audience is expected, validation passes (or fails, depending on policy)
	if expectedAudience == "" {
		return true // Or false if audience is mandatory
	}
	// If the claim is empty, it fails
	if len(claimsAudience) == 0 {
		return false
	}
	// Check if the expected audience exists within the claim slice
	for _, aud := range claimsAudience {
		if aud == expectedAudience {
			return true
		}
	}
	// Expected audience not found
	return false
}
