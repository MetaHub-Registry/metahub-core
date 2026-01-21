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
	bytes, _ := bcrypt.GenerateFromPassword([]byte(passwd), 12)
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

// Encode encodes a string
func Base64Encode(decoded string) string {
	var encoded bytes.Buffer
	encoder := base64.NewEncoder(base64.StdEncoding, &encoded)
	encoder.Write([]byte(decoded))
	encoder.Close()
	return encoded.String()
}

// Decode decodes an encoded string
func Base64Decode(encoded string) string {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
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
		res = strings.ReplaceAll(matches[1], ".", "_")
		if strings.HasSuffix(res, "+json") {
			res = strings.ReplaceAll(res, "+json", "JSON")
		}
		if strings.HasSuffix(res, "+gzip") {
			res = strings.ReplaceAll(res, "+gzip", "GZIP")
		}
		if strings.HasSuffix(res, "+zstd") {
			res = strings.ReplaceAll(res, "+zstd", "ZSTD")
		}
	} else {
		res = mediatype
	}

	return
}

func ExpandMediaType(mediatype string) string {
	mt := strings.ReplaceAll(mediatype, "_", ".")
	mt = strings.ReplaceAll(mt, ":", "+")
	if strings.HasSuffix(mt, "JSON") {
		mt = strings.ReplaceAll(mt, "JSON", "+json")
	}
	if strings.HasSuffix(mt, "GZIP") {
		mt = strings.ReplaceAll(mt, "GZIP", "+gzip")
	}
	res := fmt.Sprintf("application/vnd.%s", mt)
	if strings.HasPrefix(mt, "application/vnd.") {
		return mt
	}
	return res
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
	case
		"oci.image.index.v1+json",
		"oci.image.config.v1+json", "oci_image_manifest_v1JSON":
		label = fmt.Sprintf("oci:%s", d)
	case
		"docker_distribution_manifest_v2JSON",
		"docker.container.image.v1+json",
		"docker.image.manifest.list.v2+json":
		label = fmt.Sprintf("dkr:%s", d)
	case "oci_image_layer_v1_tarGZIP":
		label = fmt.Sprintf("ociGz:%s", d)
	case "oci_image_layer_v1_tarZSTD":
		label = fmt.Sprintf("ociZstd:%s", d)
	case "docker_image_rootfs_diff_tar_gzip":
		label = fmt.Sprintf("dkrGz:%s", d)
	case "docker_image_rootfs_diff_tar":
		label = fmt.Sprintf("dkr:%s", d)
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
