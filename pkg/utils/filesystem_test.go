package utils_test

import (
	"fmt"

	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func ExampleRelativePath() {
	res := utils.RelativePath("/tmp", "/tmp/foo/bar")
	fmt.Println(res)
	// Output: foo/bar
}
