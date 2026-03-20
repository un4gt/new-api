package nvidia

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

var initHTTPClientOnce sync.Once

func ensureHTTPClient() {
	initHTTPClientOnce.Do(service.InitHttpClient)
}

func TestNvidiaAdaptorGetRequestURL(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}

	t.Run("text embedding uses /v1/embeddings", func(t *testing.T) {
		info := &relaycommon.RelayInfo{
			RelayMode: relayconstant.RelayModeEmbeddings,
			ChannelMeta: &relaycommon.ChannelMeta{
				ChannelBaseUrl:    "https://integrate.api.nvidia.com",
				UpstreamModelName: ModelNvEmbedV1,
			},
		}
		url, err := adaptor.GetRequestURL(info)
		require.NoError(t, err)
		require.Equal(t, "https://integrate.api.nvidia.com/v1/embeddings", url)
	})

	t.Run("nv-dinov2 uses cv infer endpoint", func(t *testing.T) {
		info := &relaycommon.RelayInfo{
			RelayMode: relayconstant.RelayModeEmbeddings,
			ChannelMeta: &relaycommon.ChannelMeta{
				ChannelBaseUrl:    "https://integrate.api.nvidia.com",
				UpstreamModelName: ModelNVDinoV2,
			},
		}
		url, err := adaptor.GetRequestURL(info)
		require.NoError(t, err)
		require.Equal(t, nvDinoV2InferURL, url)
	})
}

func TestNvidiaAdaptorConvertEmbeddingRequestRejectsUnsupportedModel(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeEmbeddings,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "text-embedding-3-large",
		},
	}
	req := dto.EmbeddingRequest{
		Model: "text-embedding-3-large",
		Input: "hello",
	}
	_, err := adaptor.ConvertEmbeddingRequest(nil, info, req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported Nvidia embedding model")
}

func TestParseSingleImageInput(t *testing.T) {
	t.Parallel()

	t.Run("string input", func(t *testing.T) {
		val, err := parseSingleImageInput("https://example.com/a.png")
		require.NoError(t, err)
		require.Equal(t, "https://example.com/a.png", val)
	})

	t.Run("single element array input", func(t *testing.T) {
		val, err := parseSingleImageInput([]any{"https://example.com/a.png"})
		require.NoError(t, err)
		require.Equal(t, "https://example.com/a.png", val)
	})

	t.Run("multiple array elements should fail", func(t *testing.T) {
		_, err := parseSingleImageInput([]any{"a", "b"})
		require.Error(t, err)
	})

	t.Run("invalid type should fail", func(t *testing.T) {
		_, err := parseSingleImageInput(123)
		require.Error(t, err)
	})
}

func TestCreateNvcfAsset(t *testing.T) {
	ensureHTTPClient()

	createServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v2/nvcf/assets", r.URL.Path)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Contains(t, string(body), "\"contentType\":\"image/jpeg\"")
		require.Contains(t, string(body), "\"description\":\"Input Image\"")

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"uploadUrl":"https://upload.example.com/put","assetId":"asset-123"}`)
	}))
	defer createServer.Close()

	oldURL := nvDinoV2NvcfAssetsURL
	nvDinoV2NvcfAssetsURL = createServer.URL + "/v2/nvcf/assets"
	defer func() {
		nvDinoV2NvcfAssetsURL = oldURL
	}()

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey: "test-key",
		},
	}
	resp, err := createNvcfAsset(nil, info, "image/jpeg", "Input Image")
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "asset-123", resp.AssetID)
	require.Equal(t, "https://upload.example.com/put", resp.UploadURL)
}

func TestPutNvcfAssetBinary(t *testing.T) {
	t.Parallel()
	ensureHTTPClient()

	uploadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "/upload", r.URL.Path)
		require.Equal(t, "Input Image", r.Header.Get("x-amz-meta-nvcf-asset-description"))
		require.Equal(t, "image/png", r.Header.Get("content-type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, []byte("binary-data"), body)
		w.WriteHeader(http.StatusOK)
	}))
	defer uploadServer.Close()

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{},
	}
	err := putNvcfAssetBinary(nil, info, uploadServer.URL+"/upload", "image/png", "Input Image", []byte("binary-data"))
	require.NoError(t, err)
}

func TestUploadNvcfAssetEndToEnd(t *testing.T) {
	ensureHTTPClient()

	uploadCalled := false
	uploadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uploadCalled = true
		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "Input Image", r.Header.Get("x-amz-meta-nvcf-asset-description"))
		require.Equal(t, "image/jpeg", r.Header.Get("content-type"))
		data, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, []byte("hello"), data)
		w.WriteHeader(http.StatusOK)
	}))
	defer uploadServer.Close()

	originalCreate := createNvcfAsset
	createNvcfAsset = func(c *gin.Context, info *relaycommon.RelayInfo, mimeType string, description string) (*nvcfCreateAssetResponse, error) {
		return &nvcfCreateAssetResponse{
			UploadURL: uploadServer.URL + "/upload",
			AssetID:   "asset-end2end",
		}, nil
	}
	defer func() {
		createNvcfAsset = originalCreate
	}()

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{},
	}
	assetID, err := uploadNvcfAsset(nil, info, "aGVsbG8=", "image/jpeg", "Input Image")
	require.NoError(t, err)
	require.Equal(t, "asset-end2end", assetID)
	require.True(t, uploadCalled)
}

func TestBuildNVDinoV2RequestAndRuntimeHeadersSmallImage(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", nil)

	encoded := base64.StdEncoding.EncodeToString([]byte("small-image-data"))
	input := "data:image/png;base64," + encoded

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: ModelNVDinoV2,
		},
	}

	req, runtimeHeaders, err := buildNVDinoV2RequestAndRuntimeHeaders(ctx, info, input)
	require.NoError(t, err)
	require.NotNil(t, req)
	require.Nil(t, runtimeHeaders)
	require.Len(t, req.Messages, 1)
	require.Equal(t, "image_url", req.Messages[0].Content.Type)
	require.Equal(t, "data:image/png;base64,"+encoded, req.Messages[0].Content.ImageURL.URL)
}

func TestConvertNVDinoV2ResponseToOpenAI(t *testing.T) {
	t.Parallel()

	resp := &nvDinoV2Response{
		Metadata: []nvDinoV2Metadata{
			{Embedding: []float64{0.1, 0.2}},
			{Embedding: []float64{0.3, 0.4}},
		},
	}

	out := convertNVDinoV2ResponseToOpenAI(resp, ModelNVDinoV2, 17)
	require.Equal(t, "list", out.Object)
	require.Equal(t, ModelNVDinoV2, out.Model)
	require.Len(t, out.Data, 2)
	require.Equal(t, 17, out.Usage.PromptTokens)
	require.Equal(t, 17, out.Usage.TotalTokens)
}

func TestNVDinoV2EmbeddingHandler(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", nil)

	body := `{"metadata":[{"embedding":[0.11,0.22],"frame_num":0}],"created":1,"model":"nv-dinov2","object":"inference.completion","usage":{"inference_response_time":1}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: ModelNVDinoV2,
		},
	}
	info.SetEstimatePromptTokens(9)

	usage, apiErr := nvDinoV2EmbeddingHandler(ctx, resp, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Equal(t, 9, usage.PromptTokens)
	require.Equal(t, 9, usage.TotalTokens)
	require.Contains(t, recorder.Body.String(), "\"object\":\"list\"")
	require.Contains(t, recorder.Body.String(), "\"model\":\"nv-dinov2\"")
	require.Contains(t, recorder.Body.String(), "\"embedding\":[0.11,0.22]")
}

func TestNvidiaAdaptorDoResponseInvalidRelayMode(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeUnknown,
	}

	usage, apiErr := adaptor.DoResponse(ctx, &http.Response{Body: io.NopCloser(bytes.NewBuffer(nil))}, info)
	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeInvalidRequest, apiErr.GetErrorCode())
}

func TestNvidiaSetupRequestHeader(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", nil)
	ctx.Request.Header.Set("Content-Type", "application/json")

	headers := http.Header{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey: "nvidia-key",
		},
	}
	adaptor := &Adaptor{}
	err := adaptor.SetupRequestHeader(ctx, &headers, info)
	require.NoError(t, err)
	require.Equal(t, "Bearer nvidia-key", headers.Get("Authorization"))
	require.Equal(t, "application/json", headers.Get("Content-Type"))
	require.Equal(t, "application/json", headers.Get("Accept"))
}

func TestNvidiaConvertEmbeddingRequestForTextModelPassThrough(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeEmbeddings,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: ModelNvEmbedV1,
		},
	}
	req := dto.EmbeddingRequest{
		Model: ModelNvEmbedV1,
		Input: "hello",
	}

	out, err := adaptor.ConvertEmbeddingRequest(nil, info, req)
	require.NoError(t, err)
	emb, ok := out.(dto.EmbeddingRequest)
	require.True(t, ok)
	require.Equal(t, ModelNvEmbedV1, emb.Model)
}

func TestNvidiaModelWhitelistLength(t *testing.T) {
	t.Parallel()

	require.Len(t, ModelList, 10)
	for _, m := range ModelList {
		require.Truef(t, IsSupportedModel(m), "model %s should be in whitelist", m)
	}
}

func TestNvidiaParseImageSource(t *testing.T) {
	t.Parallel()

	urlSrc := parseImageSource("https://example.com/a.jpg")
	require.True(t, urlSrc.IsURL())
	require.Equal(t, "https://example.com/a.jpg", urlSrc.URL)

	b64Src := parseImageSource("aGVsbG8=")
	require.True(t, b64Src.IsBase64())
	require.Equal(t, "aGVsbG8=", b64Src.Base64Data)
}

func TestNvidiaNormalizeImageMimeType(t *testing.T) {
	t.Parallel()

	require.Equal(t, "image/jpeg", normalizeImageMimeType("image/jpg"))
	require.Equal(t, "image/jpeg", normalizeImageMimeType("image/jpeg"))
	require.Equal(t, "image/png", normalizeImageMimeType("image/png"))
	require.Equal(t, nvDinoV2DefaultImageMime, normalizeImageMimeType("image/webp"))
	require.Equal(t, "", normalizeImageMimeType(""))
}

func TestNvidiaConvertEmbeddingRequestLargeImageAddsRuntimeHeaders(t *testing.T) {
	originalBuilder := buildNVDinoV2RequestAndRuntimeHeaders
	buildNVDinoV2RequestAndRuntimeHeaders = func(c *gin.Context, info *relaycommon.RelayInfo, input any) (*nvDinoV2Request, map[string]any, error) {
		return &nvDinoV2Request{
				Messages: []nvDinoV2Message{
					{
						Content: nvDinoV2Content{
							Type: "image_url",
							ImageURL: nvDinoV2ImageInput{
								URL: "data:image/jpeg;asset_id,asset-abc",
							},
						},
					},
				},
			}, map[string]any{
				"nvcf-input-asset-references": "asset-abc",
				"nvcf-function-asset-ids":     "asset-abc",
			}, nil
	}
	defer func() {
		buildNVDinoV2RequestAndRuntimeHeaders = originalBuilder
	}()

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeEmbeddings,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: ModelNVDinoV2,
			HeadersOverride: map[string]any{
				"x-static": "v1",
			},
		},
	}
	req := dto.EmbeddingRequest{
		Model: ModelNVDinoV2,
		Input: "https://example.com/a.jpg",
	}

	out, err := adaptor.ConvertEmbeddingRequest(nil, info, req)
	require.NoError(t, err)
	require.NotNil(t, out)
	require.True(t, info.UseRuntimeHeadersOverride)
	require.Equal(t, "asset-abc", fmt.Sprintf("%v", info.RuntimeHeadersOverride["nvcf-input-asset-references"]))
	require.Equal(t, "asset-abc", fmt.Sprintf("%v", info.RuntimeHeadersOverride["nvcf-function-asset-ids"]))
	require.Equal(t, "v1", fmt.Sprintf("%v", info.RuntimeHeadersOverride["x-static"]))
}
