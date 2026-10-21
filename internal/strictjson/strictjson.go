// Package strictjson decodes exactly one JSON value, rejecting unknown fields
// and any trailing content other than whitespace.
package strictjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

var ErrTrailing = errors.New("multiple JSON values")

// trailingError preserves a decoder error's text while letting callers give
// trailing content a different message from a malformed first value.
type trailingError struct{ error }

func (e trailingError) Is(target error) bool { return target == ErrTrailing }
func (e trailingError) Unwrap() error        { return e.error }

func Decode(data []byte, out any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		if err != nil {
			return trailingError{err}
		}
		return ErrTrailing
	}
	return nil
}
