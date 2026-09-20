//go:build linux

package gops

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseLoadavgTasks(t *testing.T) {
	assert.Equal(t, 3385, parseLoadavgTasks("0.37 0.85 1.53 1/3385 94690\n"))
	assert.Equal(t, 0, parseLoadavgTasks("0.37 0.85 1.53"))
	assert.Equal(t, 0, parseLoadavgTasks("0.37 0.85 1.53 garbage 94690"))
}
