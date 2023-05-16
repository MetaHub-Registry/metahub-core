package utils

import (
	"fmt"
	"sort"
	"strings"

	"github.com/minio/minio-go/v7/pkg/set"
)

var (
	ERR_SPACES_IN_ENV = fmt.Errorf("spaces in env vars not allowed")
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
	key, val, found := strings.Cut(v, "=")
	if !found {
		return fmt.Errorf("no '=' found in '%s'", v)
	}
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
