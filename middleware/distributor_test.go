package middleware

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSetupContextForCloudflareDoesNotUseOtherAsAPIVersion(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", nil)
	channel := &model.Channel{
		Type:  constant.ChannelCloudflare,
		Key:   "account-id,api-token",
		Other: "legacy-account-id",
	}

	require.Nil(t, SetupContextForSelectedChannel(ctx, channel, "@cf/baai/bge-m3"))
	_, exists := ctx.Get("api_version")
	require.False(t, exists)
}

func TestGetModelRequestMultipartEmbeddingsReadsModel(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "nvidia/nv-dinov2"))
	part, err := writer.CreateFormFile("input", "sample.jpg")
	require.NoError(t, err)
	_, err = part.Write([]byte("fake-image-content"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	ctx.Request = req

	modelReq, shouldSelectChannel, err := getModelRequest(ctx)
	require.NoError(t, err)
	require.True(t, shouldSelectChannel)
	require.NotNil(t, modelReq)
	require.Equal(t, "nvidia/nv-dinov2", modelReq.Model)
}

func TestGetModelRequestMultipartEmbeddingsNonNVDinoV2KeepsOriginalBehavior(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "nvidia/nv-embed-v1"))
	part, err := writer.CreateFormFile("input", "sample.jpg")
	require.NoError(t, err)
	_, err = part.Write([]byte("fake-image-content"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	ctx.Request = req

	modelReq, shouldSelectChannel, err := getModelRequest(ctx)
	require.NoError(t, err)
	require.True(t, shouldSelectChannel)
	require.NotNil(t, modelReq)
	require.Equal(t, "", modelReq.Model)
}
