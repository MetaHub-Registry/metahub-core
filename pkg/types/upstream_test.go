package types

import "testing"

func TestNewUpstreamByName(t *testing.T) {
	tests := []struct {
		name    string
		want    Upstream
		wantErr bool
	}{
		{"registry:5050/cqnib/metahub-layers/hpc3/glow:1.5.0-arm64", UpstreamGlow150ARM64, false},
		//{"registry:5050/cqnib/metahub-layers/hpc3/glow", UpstreamGlow150ARM64, true},
		//{"registry:5050/", UpstreamGlow150ARM64, true},
	}
	for _, tt := range tests {
		got, err := NewUpstreamByName(tt.name)
		if (err != nil) != tt.wantErr {
			t.Errorf("NewUpstreamByName() error = %v, wantErr %v", err, tt.wantErr)
			continue
		}
		if tt.wantErr {
			continue
		}
		if got.GetLabel() != tt.want.GetLabel() {
			t.Errorf("NewUpstreamByName() = %v, want %v", got, tt.want)
		}

	}
}

func TestNewUpstreamFromRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     Request
		want    Upstream
		wantErr bool
	}{
		{
			"glow150arm64",
			Request{
				Repo:        "cqnib/metahub-layers/hpc3/glow",
				Tag:         "1.5.0-arm64",
				Profile:     "hpc3-glow",
				Host:        "registry:5050",
				ContentType: "application/vnd.docker.distribution.manifest.v2+json",
				Length:      123456,
				Digest:      "sha256:3f2e4c3b5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b",
			},
			UpstreamGlow150ARM64,
			false,
		},
		{
			"invalidDigest",
			Request{
				Repo:        "cqnib/metahub-layers/hpc3/glow",
				Tag:         "1.5.0-arm64",
				Profile:     "hpc3-glow",
				Host:        "registry:5050",
				ContentType: "application/vnd.docker.distribution.manifest.v2+json",
				Length:      123456,
				Digest:      "invalidDigest",
			},
			Upstream{},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.want.GetLabel(), func(t *testing.T) {
			got, err := NewUpstreamFromRequest(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewUpstreamFromRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.GetLabel() != tt.want.GetLabel() {
				t.Errorf("NewUpstreamFromRequest() = %v, want %v", got, tt.want)
			}

		})
	}
}
