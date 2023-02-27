package utils

import "regexp"

var (
	dnsRe = regexp.MustCompile(`^([a-z0-9_\-]+\.){3,8}([a-z0-9_\-]+)$`)
)

func IsReverseDNS(in string) bool {
	return dnsRe.MatchString(in)
}
