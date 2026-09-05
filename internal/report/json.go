package report

import (
	"bytes"
	"encoding/json"
	"io"
)

// DecodeJSON accepts exactly one complete JSON value, preserving numbers as
// json.Number. The same contract feeds report analysis and page templates.
func DecodeJSON(src []byte) (any, bool) {
	var value any
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(stripUTF8BOM(src))))
	dec.UseNumber()
	if err := dec.Decode(&value); err != nil {
		return nil, false
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return nil, false
	}
	return value, true
}
