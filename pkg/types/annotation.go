package types

import (
	"encoding/json"
	"fmt"

	"github.com/minio/minio-go/v7/pkg/set"
)

type Annotations struct {
	ID   int64
	Keys set.StringSet
	Map  map[string]string
}

func NewAnnotations() Annotations {
	return Annotations{
		Keys: set.NewStringSet(),
		Map:  make(map[string]string),
	}
}

func (a *Annotations) AddAnnotation(k, v string) (err error) {
	if a.Keys.Contains(k) && a.Map[k] != v {
		return fmt.Errorf("annotation %s already exists and differs from new. (current:'%s' / new: '%s'", k, a.Map[k], v)
	}
	a.Keys.Add(k)
	a.Map[k] = v
	return nil
}

func (a *Annotations) GetAnnotation(k string) (v string, err error) {
	if !a.Keys.Contains(k) {
		return "", fmt.Errorf("annotation %s does not exist", k)
	}
	return a.Map[k], nil
}

func (a Annotations) MarshalJSON() ([]byte, error) {
	res := map[string]string{}
	for t := range a.Keys {
		res[t] = a.Map[t]
	}
	return json.Marshal(res)
}

func NewAnnotationsFromMap(m map[string]string) (a Annotations, err error) {
	a = NewAnnotations()
	for k, v := range m {
		err = a.AddAnnotation(k, v)
		if err != nil {
			return
		}
	}
	return
}
