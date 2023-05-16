package utils

import (
	"fmt"
	"regexp"

	log "github.com/sirupsen/logrus"
)

func ReSubMatchMap(r *regexp.Regexp, str string) (res map[string]string, err error) {
	l := log.WithFields(log.Fields{
		"package": "utils",
		"func":    "ReSubMatchMap",
	})
	_ = l
	match := r.FindStringSubmatch(str)
	subMatchMap := make(map[string]string)
	subnames := r.SubexpNames()
	if len(subnames) != len(match) {
		err = fmt.Errorf("no match found for regexp '%s' in string '%s'", r.String(), str)
		return res, err
	}
	for i, name := range subnames {
		if i != 0 {
			subMatchMap[name] = match[i]
		}
	}
	return subMatchMap, nil
}
