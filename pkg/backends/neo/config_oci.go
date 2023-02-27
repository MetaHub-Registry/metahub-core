package neo

import (
	"context"
	"fmt"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func (n Neo) OCIv1Config_GetInsertDescriptor(ctx context.Context, desc v1.Descriptor) (cfgID int64, found bool, err error) {
	cfgID, found, err = n.Config_GetID_ByDigestAndType(ctx, desc.Digest.String(), desc.MediaType)
	if err != nil {
		return
	}
	if found {
		msg := fmt.Sprintf("Config with digest '%s' already exists", desc.Digest.String())
		logrus.Warn(msg)
		return
	}
	label := utils.CreateLabelFromDescriptor(desc)
	q := `CREATE (c:Config) SET c.digest = "%s", c.mediaType = "%s", c.size = %d, c.label ="%s" RETURN id(c) as ID`
	query := fmt.Sprintf(q, desc.Digest.String(), desc.MediaType, desc.Size, label)
	res, err := n.Execute(query)
	if err != nil {
		return
	}
	cfgID, err = res.GetInt64("ID")
	return
}
