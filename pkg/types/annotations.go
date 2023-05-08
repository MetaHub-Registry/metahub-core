package types

import (
	"encoding/json"
	"fmt"

	"github.com/minio/minio-go/v7/pkg/set"
)

type Annotations struct {
	Keys set.StringSet
	Data map[string]Annotation
}

func (a Annotations) Len() int {
	return len(a.Keys)
}

func (a Annotations) ToSlice() []Annotation {
	res := []Annotation{}
	for _, v := range a.Data {
		res = append(res, v)
	}
	return res
}
func NewAnnotations() Annotations {
	return Annotations{
		Keys: set.NewStringSet(),
		Data: make(map[string]Annotation, 0),
	}
}

func (a Annotations) ToMap() map[string]string {
	res := map[string]string{}
	for k := range a.Keys {
		res[k] = a.Data[k].Value
	}
	return res
}

func (a *Annotations) MergeAnnotations(b Annotations) (err error) {
	for k := range b.Keys {
		err = a.AddAnnotation(b.Data[k])
		if err != nil {
			return
		}
	}
	return
}

func (as *Annotations) AddNewAnnotation(k, v string) (err error) {
	a := NewAnnotation(k, v)
	err = as.AddAnnotation(a)
	return
}
func (a *Annotations) AddAnnotation(in Annotation) (err error) {
	k := in.Key
	if a.Keys.Contains(k) && a.Data[k].Value != in.Value {
		return fmt.Errorf("annotation %s already exists and differs from new. (current:'%s' / new: '%s'", k, a.Data[k].Value, in.Value)
	}
	a.Keys.Add(k)
	a.Data[k] = in
	return nil
}

func (a *Annotations) GetAnnotation(k string) (res Annotation, err error) {
	if !a.Keys.Contains(k) {
		return res, fmt.Errorf("annotation %s does not exist", k)
	}
	return a.Data[k], nil
}

func (a *Annotations) GetAnnotationValue(k string) (v string, err error) {
	if !a.Keys.Contains(k) {
		return "", fmt.Errorf("annotation %s does not exist", k)
	}
	return a.Data[k].Value, nil
}

func (a Annotations) MarshalJSON() ([]byte, error) {
	res := map[string]string{}
	for t := range a.Keys {
		res[t] = a.Data[t].Value
	}
	return json.Marshal(res)
}

func NewAnnotationsFromMap(m map[string]string) (a Annotations, err error) {
	a = NewAnnotations()
	for k, v := range m {
		anno := Annotation{
			Key:   k,
			Value: v,
		}
		err = a.AddAnnotation(anno)
		if err != nil {
			return
		}
	}
	return
}
