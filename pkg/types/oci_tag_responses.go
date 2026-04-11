package types

import (
	"encoding/json"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

type TagResponse struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

func (t TagResponse) WriteWWW(w http.ResponseWriter) {
	w.Header().Set(ContentType, "application/json")
	b, err := t.ToBytes()
	if err != nil {
		logrus.Errorf("failed to marshal manifest: %v", err)
		return
	}
	w.Write(b)
}

func (t TagResponse) ToBytes() (data []byte, err error) {
	return json.Marshal(t)
}

func (t *TagResponse) Sort() (err error) {
	t.Tags = utils.SortTags(t.Tags)
	return nil
}
