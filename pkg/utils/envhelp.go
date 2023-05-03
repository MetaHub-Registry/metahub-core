package utils

import (
	"fmt"
	"sort"
	"strings"

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

func (es EnvStore) AddEnv(v string) (err error) {
	kv := strings.Split(v, "=")
	if len(kv) != 2 {
		return fmt.Errorf("could not split '%s' into key/value", v)
	}
	key := kv[0]
	val := kv[1]
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
	keys := es.Keys.ToSlice()
	sort.Strings(keys)
	for _, k := range keys {
		v := es.Map[k]
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return
}
