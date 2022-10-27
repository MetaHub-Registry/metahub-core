package utils

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// GenPasswordHash creates a hash from a plain password
func GenPasswordHash(passwd string) string {
	bytes, _ := bcrypt.GenerateFromPassword([]byte(passwd), 14)
	return string(bytes)
}

// includesString to determine if a list include a string
func IncludesString(lookingFor string, list []string) bool {
	for _, b := range list {
		if b == lookingFor {
			return true
		}
	}
	return false
}

// StringUnion returns the union of two string lists
func StringUnion(a, b []string) []string {
	m := make(map[string]bool)

	for _, item := range a {
		m[item] = true
	}
	for _, item := range b {
		if _, ok := m[item]; !ok {
			a = append(a, item)
		}
	}
	return a
}

// Decode decodes an encoded string
func Decode(encoded string) string {
	fmt.Println(encoded)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	fmt.Println(decoded)
	if err != nil {
		return ""
	}
	return string(decoded)
}

// Return overlap in two
func FindOverlap(one []string, two []string) []string {

	var overlap []string

	// Loop through one, and see if present in two
	for _, string1 := range one {
		if IncludesString(string1, two) {
			overlap = append(overlap, string1)
		}
	}
	return overlap
}

// Return strings that are in first list, but not second
func FindMissingInSecond(one []string, two []string) []string {

	var difference []string

	// Loop through one, and see if present in two
	for _, string1 := range one {

		// It's not found in two
		if !IncludesString(string1, two) {
			difference = append(difference, string1)
		}
	}
	return difference
}

func ShortenDigest(dig string) (res string) {
	dig = strings.TrimPrefix(dig, "sha256:")
	switch {
	case len(dig) > 9:
		res = dig[:9]
	default:
		res = dig
	}
	return
}

func JsonPrettyPrint(in []byte) string {
	var out bytes.Buffer
	err := json.Indent(&out, in, "", "\t")
	if err != nil {
		return string(in)
	}
	return out.String()
}
