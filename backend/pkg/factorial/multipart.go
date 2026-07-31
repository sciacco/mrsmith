package factorial

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

// fileType is the reflect.Type of File. Hoisting it avoids a per-field
// reflect.TypeOf allocation and lets the encoders test slices' element type
// (the []File case) with a single comparison.
var fileType = reflect.TypeOf(File{})

// File is a multipart upload: a filename plus its content. Generated multipart
// body structs use File (single binary) and []File (array of binary, e.g.
// attachments[]). Nil readers are skipped (omitempty).
type File struct {
	Name   string
	Reader io.Reader
}

// doMultipart POSTs body as multipart/form-data to path and decodes into *T.
//
// body is reflected over: its exported struct fields become form fields or
// file parts named by their json tag (array params keep the [] suffix baked
// into the tag). A nil or typed-nil body yields an empty — but still valid —
// multipart body; only nil pointers and Files with nil readers are omitted.
func doMultipart[T any](ctx context.Context, c *Client, path string, body any) (*T, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := encodeBody(mw, body); err != nil {
		return nil, err
	}
	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("factorial: closing multipart body: %w", err)
	}

	requestURL := c.baseURL + apiVersionPrefix + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, &buf)
	if err != nil {
		return nil, fmt.Errorf("factorial: building request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	c.setCommonHeaders(req)

	out := new(T)
	if err := c.doRequest(req, http.MethodPost, requestURL, out); err != nil {
		return nil, err
	}
	return out, nil
}

// doMultipartUpdate PUTs body as multipart/form-data to path/id, INJECTING an
// "id" form field equal to id. Every PUT body in the spec duplicates the path
// id; generated multipart UpdateBody structs omit it, so the runtime injects
// it here — mirroring the JSON id-injection performed by doUpdate. The id
// field is written BEFORE any body fields so it sorts first when ordering
// matters to the server.
func doMultipartUpdate[T any](ctx context.Context, c *Client, path, id string, body any) (*T, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := mw.WriteField("id", id); err != nil {
		return nil, fmt.Errorf("factorial: writing id field: %w", err)
	}
	if err := encodeBody(mw, body); err != nil {
		return nil, err
	}
	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("factorial: closing multipart body: %w", err)
	}

	requestURL := c.baseURL + apiVersionPrefix + path + "/" + id
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, requestURL, &buf)
	if err != nil {
		return nil, fmt.Errorf("factorial: building request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	c.setCommonHeaders(req)

	out := new(T)
	if err := c.doRequest(req, http.MethodPut, requestURL, out); err != nil {
		return nil, err
	}
	return out, nil
}

// encodeBody reflects over body and writes its exported struct fields to mw.
// Nil/typed-nil bodies and non-struct bodies are no-ops (so a stray nil map
// cannot panic the reflection walk).
func encodeBody(mw *multipart.Writer, body any) error {
	if body == nil || isNilAny(body) {
		return nil
	}
	rv := reflect.ValueOf(body)
	for rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil
	}
	return encodeStruct(mw, rv)
}

// encodeStruct iterates the exported fields of v (a struct reflect.Value) and
// writes each as a multipart part or form field. Field names come from the
// json tag. Nil pointers and Files with nil readers are skipped; every other
// present field is encoded (no zero-value elision).
func encodeStruct(mw *multipart.Writer, v reflect.Value) error {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}
		name, skip := fieldName(sf)
		if skip {
			continue
		}
		fv := v.Field(i)
		// Dereference pointers; a nil pointer anywhere in the chain skips
		// the field entirely (omitempty semantics for *T request fields).
		skipField := false
		for fv.Kind() == reflect.Pointer {
			if fv.IsNil() {
				skipField = true
				break
			}
			fv = fv.Elem()
		}
		if skipField {
			continue
		}
		if err := encodeField(mw, name, fv); err != nil {
			return err
		}
	}
	return nil
}

// encodeField dispatches a single (dereferenced) field value to the right
// multipart encoder based on its type/kind.
func encodeField(mw *multipart.Writer, name string, fv reflect.Value) error {
	switch {
	case fv.Type() == fileType:
		return writeFile(mw, name, fv.Interface().(File))
	case fv.Kind() == reflect.Slice || fv.Kind() == reflect.Array:
		if fv.Type().Elem() == fileType {
			for j := 0; j < fv.Len(); j++ {
				if err := writeFile(mw, name, fv.Index(j).Interface().(File)); err != nil {
					return err
				}
			}
			return nil
		}
		for j := 0; j < fv.Len(); j++ {
			if err := writeField(mw, name, stringify(fv.Index(j))); err != nil {
				return err
			}
		}
		return nil
	default:
		return writeField(mw, name, stringify(fv))
	}
}

// writeFile appends f as a file part named name. Files with a nil Reader are
// skipped, matching omitempty semantics for optional binary fields.
func writeFile(mw *multipart.Writer, name string, f File) error {
	if f.Reader == nil {
		return nil
	}
	part, err := mw.CreateFormFile(name, f.Name)
	if err != nil {
		return fmt.Errorf("factorial: creating file part %q: %w", name, err)
	}
	if _, err := io.Copy(part, f.Reader); err != nil {
		return fmt.Errorf("factorial: writing file part %q: %w", name, err)
	}
	return nil
}

// writeField appends a single form field, wrapping any writer error.
func writeField(mw *multipart.Writer, name, value string) error {
	if err := mw.WriteField(name, value); err != nil {
		return fmt.Errorf("factorial: writing field %q: %w", name, err)
	}
	return nil
}

// fieldName resolves a struct field's wire name from its json tag. It returns
// skip=true for the literal `json:"-"`. The name is the tag segment before
// the first comma; an empty name (e.g. `json:",omitempty"`) or a missing tag
// falls back to the Go field name. The [] suffix of array params is preserved
// verbatim because it is baked into the tag (e.g. json:"attachments[]").
func fieldName(sf reflect.StructField) (string, bool) {
	tag := sf.Tag.Get("json")
	if tag == "-" {
		return "", true
	}
	name := tag
	if i := strings.IndexByte(tag, ','); i >= 0 {
		name = tag[:i]
	}
	if name == "" {
		return sf.Name, false
	}
	return name, false
}

// stringify renders a scalar reflect.Value as the form value for its field.
// Enum types (type X string) land in the String-kind branch and serialize as
// their underlying string, which is exactly what the API expects.
func stringify(v reflect.Value) string {
	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Bool:
		return strconv.FormatBool(v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(v.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'f', -1, 64)
	default:
		return fmt.Sprint(v.Interface())
	}
}
