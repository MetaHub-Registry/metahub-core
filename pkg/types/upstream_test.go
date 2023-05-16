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
