package magazine

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestInitOIDC_NoIssuer(t *testing.T) {
	viper.Reset()
	cfg := MagConfig{}
	err := initOIDC(&cfg)
	assert.NoError(t, err)
	assert.Equal(t, "", cfg.OIDC.Issuer, "OIDC should not be configured when issuer is empty")
}

func TestInitOIDC_WithIssuer(t *testing.T) {
	viper.Reset()
	viper.Set("oidc.issuer", "https://auth.example.com/application/o/myapp/")
	viper.Set("oidc.clientid", "my-client-id")
	viper.Set("oidc.secret", "my-client-secret")
	viper.Set("oidc.redirect_uri", "https://app.example.com/callback")
	defer viper.Reset()

	cfg := MagConfig{}
	err := initOIDC(&cfg)
	assert.NoError(t, err)
	assert.Equal(t, "https://auth.example.com/application/o/myapp/", cfg.OIDC.Issuer)
	assert.Equal(t, "my-client-id", cfg.OIDC.ClientID)
	assert.Equal(t, "my-client-secret", cfg.OIDC.ClientSecret)
	assert.Equal(t, "https://app.example.com/callback", cfg.OIDC.RedirectURI)
}
