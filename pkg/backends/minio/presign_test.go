package minio

import (
	"context"
	neturl "net/url"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// newPresignTestMinio builds a Minio whose blob bucket has an internal client and
// a (possibly distinct) presign client. Presigning is offline, so neither endpoint
// needs to be reachable.
func newPresignTestMinio(t *testing.T, internalURL, publicURL string, publicSsl bool, region string) Minio {
	t.Helper()
	bc := BucketConfig{
		URL:       internalURL,
		Region:    region,
		User:      minioAccessKey,
		Password:  minioSecretKey,
		Bucket:    blobBucketKey,
		PublicURL: publicURL,
		PublicSsl: publicSsl,
	}
	internalCli, err := minio.New(internalURL, &minio.Options{
		Creds:  credentials.NewStaticV4(bc.User, bc.Password, ""),
		Region: bc.Region,
	})
	assert.NoError(t, err)
	bc.Client = internalCli
	if publicURL != "" && publicURL != internalURL {
		pcli, e := minio.New(publicURL, &minio.Options{
			Creds:  credentials.NewStaticV4(bc.User, bc.Password, ""),
			Secure: publicSsl,
			Region: bc.Region,
		})
		assert.NoError(t, e)
		bc.PresignClient = pcli
	} else {
		bc.PresignClient = internalCli
	}
	return Minio{BucketConfig: map[string]BucketConfig{blobBucketKey: bc}}
}

const presignTestDigest = "sha256:59bf1c3509f33515622619af21ed55bbe26d24913cedbca106468a5fb37a50c3"

// A configured public endpoint is the host of the presigned URL, not the internal one.
func TestBlobGetPreSignedUrl_UsesPublicEndpoint(t *testing.T) {
	m := newPresignTestMinio(t, "minio-dev.metahub-dev.svc.cluster.local:9000", "minio.dev.example.com", false, "us-east-1")
	raw, err := m.BlobGetPreSignedUrl(context.Background(), presignTestDigest)
	assert.NoError(t, err)
	u, err := neturl.Parse(raw)
	assert.NoError(t, err)
	assert.Equal(t, "minio.dev.example.com", u.Host)
	assert.Equal(t, "http", u.Scheme)
}

// With no public endpoint (and no presign client) the signer falls back to the
// internal client — unchanged behaviour.
func TestBlobGetPreSignedUrl_FallsBackToInternal(t *testing.T) {
	m := newPresignTestMinio(t, "minio-dev.metahub-dev.svc.cluster.local:9000", "", false, "us-east-1")
	// Force the "PresignClient == nil" fallback path explicitly.
	bc := m.BucketConfig[blobBucketKey]
	bc.PresignClient = nil
	m.BucketConfig[blobBucketKey] = bc
	raw, err := m.BlobGetPreSignedUrl(context.Background(), presignTestDigest)
	assert.NoError(t, err)
	u, err := neturl.Parse(raw)
	assert.NoError(t, err)
	assert.Equal(t, "minio-dev.metahub-dev.svc.cluster.local:9000", u.Host)
}

// An AWS S3 TLS endpoint yields an https presigned URL addressed to the regional
// S3 host (virtual-hosted style: <bucket>.s3.<region>.amazonaws.com).
func TestBlobGetPreSignedUrl_S3HttpsHost(t *testing.T) {
	m := newPresignTestMinio(t, "minio-dev.metahub-dev.svc.cluster.local:9000", "s3.eu-central-1.amazonaws.com", true, "eu-central-1")
	raw, err := m.BlobGetPreSignedUrl(context.Background(), presignTestDigest)
	assert.NoError(t, err)
	u, err := neturl.Parse(raw)
	assert.NoError(t, err)
	assert.Equal(t, "https", u.Scheme)
	// minio-go derives the AWS host from the region option (virtual-hosted,
	// dualstack) — e.g. blobs.s3.dualstack.eu-central-1.amazonaws.com.
	assert.Contains(t, u.Host, "eu-central-1.amazonaws.com")
}

// ConnectClients builds a DISTINCT presign client when the public endpoint
// differs from the internal one.
func TestConnectClients_PresignClientDistinct(t *testing.T) {
	m := Minio{BucketConfig: map[string]BucketConfig{
		blobBucketKey: {
			Bucket: blobBucketKey, URL: "internal.local:9000", Region: "us-east-1",
			User: minioAccessKey, Password: minioSecretKey,
			PublicURL: "minio.dev.example.com", PublicSsl: false,
		},
	}}
	err := m.ConnectClients()
	assert.NoError(t, err)
	bc := m.BucketConfig[blobBucketKey]
	assert.NotNil(t, bc.Client)
	assert.NotNil(t, bc.PresignClient)
	assert.NotSame(t, bc.Client, bc.PresignClient)
}

// ConnectClients reuses the internal client for presigning when no distinct
// public endpoint is configured (identity).
func TestConnectClients_PresignClientIdentity(t *testing.T) {
	m := Minio{BucketConfig: map[string]BucketConfig{
		blobBucketKey: {
			Bucket: blobBucketKey, URL: "internal.local:9000", Region: "us-east-1",
			User: minioAccessKey, Password: minioSecretKey,
		},
	}}
	err := m.ConnectClients()
	assert.NoError(t, err)
	bc := m.BucketConfig[blobBucketKey]
	assert.Same(t, bc.Client, bc.PresignClient)
}

// NewMinio resolves per-bucket public_url, the global minio.public_url fallback,
// and finally the internal url; global public_ssl applies when set.
func TestNewMinio_PublicEndpointResolution(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	viper.Set("minio.url", "minio-dev.metahub-dev.svc.cluster.local:9000")
	viper.Set("minio.region", "eu-central-1")
	viper.Set("minio.public_ssl", true)
	viper.Set("minio.blobs.bucket", "metahub-api-dev")
	viper.Set("minio.blobs.public_url", "minio.dev.example.com")
	viper.Set("minio.blobs.redirect", true)
	viper.Set("minio.manifests.bucket", "metahub-api-dev")

	m, err := NewMinio("/tmp/unused-presign-test.yaml")
	assert.NoError(t, err)
	// blobs: explicit per-bucket public_url wins.
	assert.Equal(t, "minio.dev.example.com", m.BucketConfig[blobBucketKey].PublicURL)
	// manifests: no per-bucket/global public_url → falls back to internal url.
	assert.Equal(t, "minio-dev.metahub-dev.svc.cluster.local:9000", m.BucketConfig[manifestsBucketKey].PublicURL)
	// global public_ssl applies to buckets without their own.
	assert.True(t, m.BucketConfig[blobBucketKey].PublicSsl)
	// per-bucket redirect is honoured (covers the viperBool set-path).
	assert.True(t, m.BucketConfig[blobBucketKey].Redirect)
	assert.False(t, m.BucketConfig[manifestsBucketKey].Redirect)
}
