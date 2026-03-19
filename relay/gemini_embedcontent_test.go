package relay

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCountPDFPagesFromBase64_TypePageHeuristic(t *testing.T) {
	t.Parallel()

	// Not a full valid PDF; sufficient for the heuristic which scans for "/Type /Page".
	fakePDF := []byte("%PDF-1.4\n" + strings.Repeat("<< /Type /Page >>\n", 7))
	b64 := base64.StdEncoding.EncodeToString(fakePDF)

	pages, determined, err := countPDFPagesFromBase64(b64, 6)
	require.NoError(t, err)
	require.True(t, determined)
	require.Equal(t, 7, pages)
}

func TestCountPDFPagesFromBase64_PagesCountHeuristic(t *testing.T) {
	t.Parallel()

	// Prefer /Type /Pages + /Count when present.
	fakePDF := []byte("%PDF-1.4\n<< /Type /Pages /Count 9 >>\n")
	b64 := base64.StdEncoding.EncodeToString(fakePDF)

	pages, determined, err := countPDFPagesFromBase64(b64, 6)
	require.NoError(t, err)
	require.True(t, determined)
	require.Equal(t, 9, pages)
}
