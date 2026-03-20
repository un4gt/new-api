package dto

import (
	"strings"

	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type RerankMultimodalItem struct {
	Text  *string `json:"text,omitempty"`
	Image *string `json:"image,omitempty"`
}

type RerankMultimodalRequest struct {
	Model           string                 `json:"model"`
	Query           RerankMultimodalItem   `json:"query"`
	Documents       []RerankMultimodalItem `json:"documents"`
	ReturnDocuments *bool                  `json:"return_documents,omitempty"`
}

func (r *RerankMultimodalRequest) GetTokenCountMeta() *types.TokenCountMeta {
	texts := make([]string, 0, len(r.Documents)+1)
	files := make([]*types.FileMeta, 0)

	appendItemTokenMeta(&texts, &files, r.Query)
	for _, document := range r.Documents {
		appendItemTokenMeta(&texts, &files, document)
	}

	return &types.TokenCountMeta{
		CombineText: strings.Join(texts, "\n"),
		Files:       files,
	}
}

func (r *RerankMultimodalRequest) IsStream(c *gin.Context) bool {
	return false
}

func (r *RerankMultimodalRequest) SetModelName(modelName string) {
	if modelName != "" {
		r.Model = modelName
	}
}

func (r *RerankMultimodalRequest) GetReturnDocuments() bool {
	if r.ReturnDocuments == nil {
		return false
	}
	return *r.ReturnDocuments
}

func appendItemTokenMeta(texts *[]string, files *[]*types.FileMeta, item RerankMultimodalItem) {
	if item.Text != nil && *item.Text != "" {
		*texts = append(*texts, *item.Text)
	}
	if item.Image == nil || *item.Image == "" {
		return
	}
	source := newImageFileSource(*item.Image)
	*files = append(*files, types.NewImageFileMeta(source, "auto"))
}

func newImageFileSource(image string) *types.FileSource {
	if strings.HasPrefix(image, "http://") || strings.HasPrefix(image, "https://") {
		return types.NewURLFileSource(image)
	}
	return types.NewBase64FileSource(image, "")
}
