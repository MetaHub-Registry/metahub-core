package types

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

type Result struct {
	Strings map[string]string `json:"strings"`
	Ints    map[string]int    `json:"ints"`
	Int64s  map[string]int64  `json:"int64s"`
}

func NewResult() Result {
	return Result{
		Strings: make(map[string]string),
		Int64s:  make(map[string]int64),
	}
}

func (r *Result) Add(key string, val interface{}) {
	switch val.(type) {
	case string:
		r.Strings[key] = val.(string)
	case int64:
		r.Int64s[key] = val.(int64)
	case []interface{}:
		v := val.([]interface{})
		switch v[0].(type) {
		case string:
			str := v[0].(string)
			logrus.Warnf("  > %s: %s", key, str)
			r.Strings[key] = str
		}
	default:
		panic(fmt.Sprintf("unknown type for key '%s': %T", key, val))
	}
}

func (r Result) GetStr(key string) (val string, err error) {
	if v, ok := r.Strings[key]; ok {
		return v, nil
	}
	return "", fmt.Errorf("key '%s' not found", key)
}

func (r Result) CheckKeys(keys ...string) error {
	errs := []string{}
	for _, k := range keys {
		_, StrOK := r.Strings[k]
		_, Int64OK := r.Int64s[k]
		if !StrOK && !Int64OK {
			errs = append(errs, k)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("Key(s) not found: %s", strings.Join(errs, ","))
	}
	return nil
}

func (r *Result) GetInt64(key string) (val int64, err error) {
	if v, ok := r.Int64s[key]; ok {
		return v, nil
	}
	return 0, fmt.Errorf("key '%s' not found", key)
}

func (r *Result) GetInt(key string) (val int, err error) {
	v, err := r.GetInt64(key)
	if err != nil {
		return 0, err
	}
	return int(v), nil
}
