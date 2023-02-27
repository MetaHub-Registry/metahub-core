package utils

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
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

func ShortenMediaType(mediatype string) (res string) {
	r := regexp.MustCompile(`application\/vnd.(.*)`)
	matches := r.FindStringSubmatch(mediatype)
	if len(matches) > 1 {
		res = matches[1]
	} else {
		res = mediatype
	}
	return
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

func CreateLabelFromDescriptor(desc v1.Descriptor) string {
	return CreateLabelFromTypeAndDigest(desc.MediaType, desc.Digest.String())
}

func CreateLabelFromTypeAndDigest(md, dig string) string {
	label := ShortenMediaType(md)
	d := ShortenDigest(dig)
	switch label {
	case "oci.image.index.v1+json":
		label = fmt.Sprintf("oci:%s", d)
	case "oci.image.config.v1+json":
		label = fmt.Sprintf("oci:%s", d)
	case "oci.image.manifest.v1+json":
		label = fmt.Sprintf("oci:%s", d)
	case "docker.distribution.manifest.v2+json":
		label = fmt.Sprintf("dkr:%s", d)
	case "docker.image.manifest.list.v2+json":
		label = fmt.Sprintf("dkr:%s", d)
	case "oci.image.layer.v1.tar+gzip":
		label = fmt.Sprintf("ociGz:%s", d)
	case "oci.image.layer.v1.tar":
		label = fmt.Sprintf("ociTar:%s", d)
	case "docker.container.image.v1+json":
		label = fmt.Sprintf("dkr:%s", d)
	case "docker.image.rootfs.diff.tar.gzip":
		label = fmt.Sprintf("dkrGz:%s", d)
	case "docker.image.rootfs.diff.tar":
		label = fmt.Sprintf("dkrTar:%s", d)
	default:
		label = fmt.Sprintf("??:%s", d)
	}
	return label
}

func UpLabelKey(key string) string {
	res := strings.ToUpper(key)
	res = strings.ReplaceAll(res, ".", "_")
	return res
}
