package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/nvidia"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestFetchChannelUpstreamModelIDsNvidiaReturnsWhitelist(t *testing.T) {
	t.Parallel()

	channel := &model.Channel{
		Type: constant.ChannelTypeNvidia,
	}
	models, err := fetchChannelUpstreamModelIDs(channel)
	require.NoError(t, err)
	require.Equal(t, nvidia.ModelList, models)
}

func TestFetchModelsNvidiaReturnsWhitelist(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	body := `{"type":59,"key":"test-key"}`
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/channel/fetch_models", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	FetchModels(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp struct {
		Success bool     `json:"success"`
		Data    []string `json:"data"`
	}
	err := common.Unmarshal(recorder.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.Equal(t, nvidia.ModelList, resp.Data)
}
