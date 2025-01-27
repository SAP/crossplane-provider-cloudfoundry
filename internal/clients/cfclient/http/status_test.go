package http

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStatus(t *testing.T) {
	require.True(t, isStatusIn(http.StatusOK, http.StatusAccepted, http.StatusOK))
	require.True(t, isStatusIn(http.StatusAccepted, http.StatusAccepted, http.StatusOK))
	require.False(t, isStatusIn(http.StatusNoContent, http.StatusAccepted, http.StatusOK))
}
