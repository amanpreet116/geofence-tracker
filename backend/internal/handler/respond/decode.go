package respond

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"reflect"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
)

// MaxBodyBytes caps request bodies.
const MaxBodyBytes = 1 << 20 // 1 MiB

// Decode reads a single JSON object from r into dst. It requires
// Content-Type: application/json and a body of at most MaxBodyBytes. Unknown
// fields are ignored on purpose (lenient input). Returned errors are already
// model errors, so handlers can pass them straight to Error.
func Decode(w http.ResponseWriter, r *http.Request, dst any) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return fmt.Errorf("Content-Type must be application/json: %w", model.ErrUnsupportedMediaType)
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		return decodeError(err)
	}
	// Reject trailing content such as "{}{}" or "{} garbage".
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if _, tooLarge := errors.AsType[*http.MaxBytesError](err); tooLarge {
			return decodeError(err)
		}
		return model.NewValidationError("request body must contain a single JSON object", nil)
	}
	return nil
}

func decodeError(err error) error {
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		return fmt.Errorf("request body must not exceed %d bytes: %w", MaxBodyBytes, model.ErrPayloadTooLarge)
	}
	if se, ok := errors.AsType[*json.SyntaxError](err); ok {
		return model.NewValidationError(fmt.Sprintf("malformed JSON at position %d", se.Offset), nil)
	}
	if te, ok := errors.AsType[*json.UnmarshalTypeError](err); ok {
		if te.Field == "" {
			return model.NewValidationError("request body must be a JSON object", nil)
		}
		return model.NewValidationError(
			fmt.Sprintf("invalid type for field %q", te.Field),
			map[string]string{te.Field: fmt.Sprintf("must be of type %s", jsonTypeName(te))},
		)
	}
	switch {
	case errors.Is(err, io.EOF):
		return model.NewValidationError("request body is required", nil)
	case errors.Is(err, io.ErrUnexpectedEOF):
		return model.NewValidationError("malformed JSON: unexpected end of input", nil)
	}
	return model.NewValidationError("invalid request body", nil)
}

// jsonTypeName describes the expected type in JSON terms, not Go terms.
func jsonTypeName(te *json.UnmarshalTypeError) string {
	switch te.Type.Kind() {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Map, reflect.Struct:
		return "object"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "number"
	default:
		return te.Type.String()
	}
}
