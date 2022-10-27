package utils

import (
	"regexp"

	log "github.com/sirupsen/logrus"
)

func ReSubMatchMap(r *regexp.Regexp, str string) map[string]string {
	l := log.WithFields(log.Fields{
		"package": "utils",
		"func":    "ReSubMatchMap",
	})
	match := r.FindStringSubmatch(str)
	subMatchMap := make(map[string]string)
	subnames := r.SubexpNames()
	if len(subnames) != len(match) {
		l.Warnf("Match: %v\nSubnames: %v", match, subnames)
		l.Errorf("No match found for regexp '%s' in string '%s'", r.String(), str)
	}
	for i, name := range subnames {
		if i != 0 {
			subMatchMap[name] = match[i]
		}
	}
	return subMatchMap
}
