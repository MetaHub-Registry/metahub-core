package regcli

import (
	"os"
	"strings"

	"github.com/regclient/regclient"
	"github.com/regclient/regclient/config"
	"github.com/sirupsen/logrus"
)

func NewRegClient(reg string) *regclient.RegClient {
	l := logrus.WithFields(logrus.Fields{
		"package":  "regcli",
		"func":     "NewRegClient",
		"registry": reg,
	})
	l.Debug("Create new regclient")
	var rc *regclient.RegClient
	// Deal with TLS
	if strings.HasPrefix(reg, "localhost") || reg == "registry:5050" || strings.HasPrefix(reg, "172.") || os.Getenv("TEST_REGISTRY") == "true" {
		rcHost := config.Host{
			Name:     reg,
			Hostname: reg,
			TLS:      config.TLSDisabled,
		}
		rc = regclient.New(
			regclient.WithConfigHost(rcHost),
		)
	} else {
		rc = regclient.New()
	}
	return rc

}
