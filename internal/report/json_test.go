package report

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodeJSONContract(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		source string
		value  any
		valid  bool
	}{
		{`9007199254740993`, json.Number("9007199254740993"), true},
		{`null`, nil, true},
		{`false`, false, true},
		{`"text"`, "text", true},
		{"\xef\xbb\xbf {} \n", map[string]any{}, true},
		{" \xef\xbb\xbf{}", nil, false},
		{`{} []`, nil, false},
		{`{} trailing`, nil, false},
		{`{"missing":`, nil, false},
		{"", nil, false},
	} {
		value, ok := DecodeJSON([]byte(tt.source))
		assert.Equal(t, tt.valid, ok, tt.source)
		assert.Equal(t, tt.value, value, tt.source)
	}
}
