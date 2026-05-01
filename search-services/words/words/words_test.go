package words

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWords(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		output []string
	}{
		{
			name:   "empty string",
			input:  "",
			output: []string{},
		},
		{
			name:   "only stop words",
			input:  "the and is",
			output: []string{},
		},
		{
			name:   "mixed case and punctuation",
			input:  "Hello, World! This is a test.",
			output: []string{"hello", "world", "test"},
		},
		{
			name:   "duplicate words",
			input:  "run running runs",
			output: []string{"run"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Norm(tt.input)
			require.Equal(t, tt.output, result)
		})
	}
}
