package types

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/docker/distribution"
	"github.com/opencontainers/go-digest"
	v1 "github.com/regclient/regclient/types/oci/v1"
	"github.com/sirupsen/logrus"
)

type Config struct {
	NeoID     int64    `json:"neo_id"`
	SqlID     int64    `json:"sql_id"`
	Data      []byte   `json:"-"`
	Digest    string   `json:"digest"`
	Label     string   `json:"label"`
	MediaType string   `json:"media_type"`
	Size      int64    `json:"size"`
	Cfg       v1.Image `json:"cfg"`
}

func NewConfigFromBytes(data []byte) (res Config, err error) {
	l := logrus.WithFields(logrus.Fields{
		"packrusage": "types/config.go",
		"func":       "NewConfigFromBytes",
	})
	err = json.Unmarshal(data, &res.Cfg)
	if err != nil {
		l.Error(err)
	}
	errs := []string{}
	if res.Cfg.Architecture == "" {
		errs = append(errs, "architecture")
	}
	if res.Cfg.OS == "" {
		errs = append(errs, "os")
	}
	if len(errs) > 0 {
		err = fmt.Errorf("config object has missing fields: %s", strings.Join(errs, ", "))
		l.Error(err)
		return
	}
	res.MediaType = "application/vnd.docker.container.image.v1+json"
	res.Digest = digest.FromBytes(data).String()
	res.Data = data
	res.Size = int64(len(data))
	return
}

func (c Config) ToDescriptor() distribution.Descriptor {
	return distribution.Descriptor{
		Digest:    digest.Digest(c.Digest),
		MediaType: c.MediaType,
		Size:      c.Size,
	}
}

func (c Config) WriteWWWHeader(w http.ResponseWriter) {
	b := c.Data
	size := len(b)
	w.Header().Set("Content-Type", c.MediaType)
	w.Header().Set("Content-Length", fmt.Sprint(size))
	w.Header().Set("Docker-Content-Digest", c.Digest)
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	w.Header().Set("Etag", c.Digest)
}

func (c Config) WriteWWWBody(w http.ResponseWriter) {
	w.Write(c.Data)
}
