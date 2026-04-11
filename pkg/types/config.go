package types

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/pkg/errors"
	v1 "github.com/regclient/regclient/types/oci/v1"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/utils"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	dockerConfigMediaType = "application/vnd.docker.container.image.v1+json"
	ociConfigMediaType    = "application/vnd.oci.image.config.v1+json"
)

type Config struct {
	ID         int64       `json:"-"`
	Data       []byte      `json:"-"`
	Digest     string      `json:"digest"`
	GraphLabel string      `json:"graph_label"`
	MediaType  string      `json:"mediaType"`
	Size       int64       `json:"size"`
	Cfg        v1.Image    `json:"cfg"`
	Labels     Annotations `json:"labels"`
}

func NewConfigFromBytes(ctx context.Context, data []byte) (res Config, err error) {
	l, ctx, span := utils.InitLogging(ctx, tracer, map[string]any{
		"package": "types",
		"func":    "NewConfigFromBytes",
	})
	defer span.End()
	_ = l
	err = json.Unmarshal(data, &res.Cfg)
	if err != nil {
		l.Error(err)
	}
	span.AddEvent("data", trace.WithAttributes(
		attribute.String("res", fmt.Sprintf("%v", res.Cfg))),
	)
	errs := []string{}
	if res.Cfg.Architecture == "" {
		errs = append(errs, "architecture")
	}
	if res.Cfg.OS == "" {
		errs = append(errs, "os")
	}
	if len(errs) > 0 {
		err = fmt.Errorf("config object has missing fields: %s", strings.Join(errs, ", "))
		span.SetStatus(codes.Error, err.Error())
		l.Error(err)
		return
	}
	res.MediaType = dockerConfigMediaType
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
	w.Header().Set(ContentType, c.MediaType)
	w.Header().Set(ContentLength, fmt.Sprint(size))
	w.Header().Set(DockerContentDigest, c.Digest)
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	w.Header().Set("Etag", c.Digest)
}

func (c Config) WriteWWW(w http.ResponseWriter) {
	b := c.Data
	size := len(b)
	w.Header().Set(ContentType, c.MediaType)
	w.Header().Set(ContentLength, fmt.Sprint(size))
	w.Header().Set(DockerContentDigest, c.Digest)
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
	w.Header().Set("Etag", c.Digest)
	w.Write(c.Data)
}

func ConfigFromOCISpecV1Image(v1Img imgspecv1.Image) (res Config) {
	res.MediaType = dockerConfigMediaType
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

func (cfg *Config) SwitchMediaType(bu *OCIBlobUpload, buf []byte) (err error) {
	if bu.MediaType != "" && cfg.MediaType != bu.MediaType {
		msg := fmt.Sprintf("Mismatched media types: cfg='%s' / bu='%s'", cfg.MediaType, bu.MediaType)
		logrus.Errorf(msg)
		return fmt.Errorf(msg)

	}
	switch bu.MediaType {
	case dockerConfigMediaType:
		var imgCfg v1.Image
		err = json.Unmarshal(buf, &imgCfg)
		if err != nil {
			logrus.Error(err)
			return
		}
		cfg.Cfg.Config.Labels = imgCfg.Config.Labels
		anno, err := NewAnnotationsFromMap(imgCfg.Config.Labels)
		if err != nil {
			logrus.Error(err)
			return err
		}
		cfg.Labels = anno

	case ociConfigMediaType:
		var imgCfg v1.Image
		err = json.Unmarshal(buf, &imgCfg)
		if err != nil {
			logrus.Error(err)
			logrus.Errorf("Failed to unmarshal JSON: %s", string(buf))
			return
		}
		anno, err := NewAnnotationsFromMap(imgCfg.Config.Labels)
		if err != nil {
			logrus.Error(err)
			return err
		}
		cfg.Cfg.Config.Labels = imgCfg.Config.Labels
		cfg.Labels = anno
	case "":
		// OCI conformance test does not set the media type
		bu.MediaType = ociConfigMediaType
		cfg.MediaType = bu.MediaType
		var imgCfg v1.Image
		err = json.Unmarshal(buf, &imgCfg)
		if err != nil {
			logrus.Error(err)
			return
		}
		anno, err := NewAnnotationsFromMap(imgCfg.Config.Labels)
		if err != nil {
			logrus.Error(err)
			return err
		}
		cfg.Cfg.Config.Labels = imgCfg.Config.Labels
		cfg.Labels = anno
	default:
		err := errors.Errorf("unknown media type: %s", bu.MediaType)
		logrus.Error(err)
		return err
	}
	return nil
}
