package utils

import (
	"fmt"

	"github.com/minio/minio-go/v7/pkg/set"
)

type EnvStore struct {
	Keys set.StringSet
	Map  map[string]string
	//envKeys := set.NewStringSet()
	//envMap := map[string]string{}
}

func NewEnvStore() EnvStore {
	return EnvStore{
		Keys: set.NewStringSet(),
		Map:  map[string]string{},
	}
}

func (es EnvStore) Add(key, val string) (err error) {
	upKey := UpLabelKey(key)
	if es.Keys.Contains(key) {
		if es.Map[key] != val {
			return fmt.Errorf("key '%s' already present with different value: '%s' vs '%s'", key, es.Map[key], val)
		}
		// Key's already there and the value is the same, so just
		return
	}
	es.Keys.Add(upKey)
	es.Map[upKey] = val
	return
}

func (es EnvStore) ToSlice() (env []string) {
	for k, v := range es.Map {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return
}
