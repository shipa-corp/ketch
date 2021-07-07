package deploy

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrors(t *testing.T) {
	require.True(t, isMissing(newMissingError("oops")))
	require.False(t, isValid(newInvalidError("oops")))
	require.True(t, isMissing(fmt.Errorf("some error %w", newMissingError("oops"))))
	require.False(t, isValid(fmt.Errorf("some error %w", newInvalidError("oops"))))
}
