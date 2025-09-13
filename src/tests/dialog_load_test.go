package tests

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDialogLoadExample(t *testing.T) {
	filename := "results.txt"
	data := []byte("some results data")

	err := os.WriteFile(filename, data, 0644)
	require.NoError(t, err)

	fmt.Printf("Results saved to %s\n", filename)
}
