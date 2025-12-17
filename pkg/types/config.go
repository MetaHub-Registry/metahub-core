package types

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	v1 "github.com/regclient/regclient/types/oci/v1"
	"github.com/sirupsen/logrus"
)

type Config struct {
	ID         int64       `json:"-"`
	Data       []byte      `json:"-"`
	Digest     string      `json:"digest"`
	GraphLabel string      `json:"graph_label"`
	MediaType  string      `json:"media_type"`
	Size       int64       `json:"size"`
	Cfg        v1.Image    `json:"cfg"`
	Labels     Annotations `json:"labels"`
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

func (c Config) ToDescriptor() imgspecv1.Descriptor {
	return imgspecv1.Descriptor{
		Digest:    digest.Digest(c.Digest),
		MediaType: c.MediaType,
		Size:      c.Size,
		Data:      c.Data,
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

func (c Config) WriteWWW(w http.ResponseWriter) {
	b := c.Data
	size := len(b)
	w.Header().Set("Content-Type", c.MediaType)
	w.Header().Set("Content-Length", fmt.Sprint(size))
	w.Header().Set("Docker-Content-Digest", c.Digest)
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	w.Header().Set("Etag", c.Digest)
	w.Write(c.Data)
}

func ConfigFromOCISpecV1Image(v1Img imgspecv1.Image) (res Config) {
	res.MediaType = "application/vnd.docker.container.image.v1+json"
	v1ImageConfig := v1.ImageConfig{
		User:         v1Img.Config.User,
		ExposedPorts: v1Img.Config.ExposedPorts,
		Env:          v1Img.Config.Env,
		Entrypoint:   v1Img.Config.Entrypoint,
		Cmd:          v1Img.Config.Cmd,
		Volumes:      v1Img.Config.Volumes,
		WorkingDir:   v1Img.Config.WorkingDir,
		Labels:       v1Img.Config.Labels,
		StopSignal:   v1Img.Config.StopSignal,
	}
	v1History := []v1.History{}
	for _, h := range v1Img.History {
		v1History = append(v1History, v1.History{
			Created:    h.Created,
			CreatedBy:  h.CreatedBy,
			Author:     h.Author,
			Comment:    h.Comment,
			EmptyLayer: h.EmptyLayer,
		})
	}
	res.Cfg = v1.Image{
		Created:      v1Img.Created,
		Author:       v1Img.Author,
		Architecture: v1Img.Architecture,
		OS:           v1Img.OS,
		Variant:      v1Img.Variant,
		OSVersion:    v1Img.OSVersion,
		OSFeatures:   v1Img.OSFeatures,
		Config:       v1ImageConfig,
		RootFS: v1.RootFS{
			Type:    v1Img.RootFS.Type,
			DiffIDs: v1Img.RootFS.DiffIDs,
		},
		History: v1History,
	}
	return
}

func (c Config) LabelsToAnnotations() (anno Annotations, err error) {
	anno = NewAnnotations()
	for k, v := range c.Cfg.Config.Labels {
		a := NewAnnotation(k, v)
		err = anno.AddAnnotation(a)
		if err != nil {
			return
		}
	}
	for _, a := range c.Labels.Data {
		err = anno.AddAnnotation(a)
		if err != nil {
			return
		}
	}
	return
}

func (c Config) ToBytes() (b []byte, err error) {
	for k, a := range c.Labels.Data {
		c.Cfg.Config.Labels[k] = a.Value
	}
	b, err = json.Marshal(c.Cfg)
	return
}

func (c Config) ToOCISpecV1Image() (res imgspecv1.Image) {
	h := []imgspecv1.History{}
	for _, h1 := range c.Cfg.History {
		h = append(h, imgspecv1.History{
			Created:    h1.Created,
			CreatedBy:  h1.CreatedBy,
			Comment:    h1.Comment,
			EmptyLayer: h1.EmptyLayer,
		})
	}
	return imgspecv1.Image{
		Created: c.Cfg.Created,
		Author:  c.Cfg.Author,
		Platform: imgspecv1.Platform{
			Architecture: c.Cfg.Architecture,
			OS:           c.Cfg.OS,
			Variant:      c.Cfg.Variant,
			OSVersion:    c.Cfg.OSVersion,
			OSFeatures:   c.Cfg.OSFeatures,
		},
		Config: imgspecv1.ImageConfig{
			User:         c.Cfg.Config.User,
			ExposedPorts: c.Cfg.Config.ExposedPorts,
			Env:          c.Cfg.Config.Env,
			Entrypoint:   c.Cfg.Config.Entrypoint,
			Cmd:          c.Cfg.Config.Cmd,
			Volumes:      c.Cfg.Config.Volumes,
			WorkingDir:   c.Cfg.Config.WorkingDir,
			Labels:       c.Cfg.Config.Labels,
			StopSignal:   c.Cfg.Config.StopSignal,
		},

		RootFS: imgspecv1.RootFS{
			Type:    c.Cfg.RootFS.Type,
			DiffIDs: c.Cfg.RootFS.DiffIDs,
		},
		History: h,
	}
}
