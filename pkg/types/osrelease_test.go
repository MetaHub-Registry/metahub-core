package types

import "testing"

const (
	u22041 = `PRETTY_NAME="Ubuntu 22.04.1 LTS"
NAME="Ubuntu"
VERSION_ID="22.04"
VERSION="22.04.1 LTS (Jammy Jellyfish)"
VERSION_CODENAME=jammy
ID=ubuntu
ID_LIKE=debian
HOME_URL="https://www.ubuntu.com/"
SUPPORT_URL="https://help.ubuntu.com/"
BUG_REPORT_URL="https://bugs.launchpad.net/ubuntu/"
PRIVACY_POLICY_URL="https://www.ubuntu.com/legal/terms-and-policies/privacy-policy"
UBUNTU_CODENAME=jammy`
	deb11 = `PRETTY_NAME="Debian GNU/Linux 11 (bullseye)"
NAME="Debian GNU/Linux"
VERSION_ID="11"
VERSION="11 (bullseye)"
VERSION_CODENAME=bullseye
ID=debian
HOME_URL="https://www.debian.org/"
SUPPORT_URL="https://www.debian.org/support"
BUG_REPORT_URL="https://bugs.debian.org/"`
	alp3162 = `NAME="Alpine Linux"
ID=alpine
VERSION_ID=3.16.2
PRETTY_NAME="Alpine Linux v3.16"
HOME_URL="https://alpinelinux.org/"
BUG_REPORT_URL="https://gitlab.alpinelinux.org/alpine/aports/-/issues"`
)

func TestOSRelease(t *testing.T) {
	tests := []struct {
		fcont string
		exp   OS
	}{
		{u22041, OS{Major: 22, Minor: 4, Patch: 0, Name: "Ubuntu", ID: "ubuntu", IDLike: "debian", Codename: "jammy"}},
		{deb11, OS{Major: 11, Minor: 0, Patch: 0, Name: "Debian GNU/Linux", ID: "debian", IDLike: "", Codename: "bullseye"}},
		{alp3162, OS{Major: 3, Minor: 16, Patch: 2, Name: "Alpine Linux", ID: "alpine", IDLike: "", Codename: ""}},
	}
	for _, test := range tests {
		os, e := ParseOS(test.fcont)
		if e != nil {
			t.Log(test.fcont)
			t.Fatalf("Failed to parse OS: %v", e)
		}
		if os.Major != test.exp.Major {
			t.Log(test.fcont)
			t.Fatalf("Major version mismatch: %d != %d", os.Major, test.exp.Major)
		}
		if os.Minor != test.exp.Minor {
			t.Log(test.fcont)
			t.Fatalf("Minor version mismatch: %d != %d", os.Minor, test.exp.Minor)
		}
		if os.Patch != test.exp.Patch {
			t.Log(test.fcont)
			t.Fatalf("Patch version mismatch: %d != %d", os.Patch, test.exp.Patch)
		}
		if os.Name != test.exp.Name {
			t.Log(test.fcont)
			t.Fatalf("Name mismatch: %s != %s", os.Name, test.exp.Name)
		}
		if os.ID != test.exp.ID {
			t.Log(test.fcont)
			t.Fatalf("ID mismatch: %s != %s", os.ID, test.exp.ID)
		}
		if os.IDLike != test.exp.IDLike {
			t.Log(test.fcont)
			t.Fatalf("IDLike mismatch: %s != %s", os.IDLike, test.exp.IDLike)
		}
		if os.Codename != test.exp.Codename {
			t.Log(test.fcont)
			t.Fatalf("Codename mismatch: %s != %s", os.Codename, test.exp.Codename)
		}
	}

}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		v   string
		exp OS
	}{
		{"22.04", OS{Major: 22, Minor: 4, Patch: 0}},
		{"22.04.1", OS{Major: 22, Minor: 4, Patch: 1}},
		{"22", OS{Major: 22, Minor: 0, Patch: 0}},
	}
	for _, test := range tests {
		major, minor, patch, err := ParseVersion(test.v)
		if err != nil {
			t.Errorf("Failed to parse version '%s': %v", test.v, err)
		}
		if major != test.exp.Major {
			t.Errorf("Major version mismatch: %d != %d", major, test.exp.Major)
		}
		if minor != test.exp.Minor {
			t.Errorf("Minor version mismatch: %d != %d", minor, test.exp.Minor)
		}
		if patch != test.exp.Patch {
			t.Errorf("Patch version mismatch: %d != %d", patch, test.exp.Patch)
		}
	}
}

func TestGetVersion(t *testing.T) {
	tests := []struct {
		in  OS
		exp string
	}{
		{OS{Major: 22, Minor: 4, Patch: 1}, "22.4.1"},
		{OS{Major: 22, Minor: 4, Patch: 0}, "22.4"},
		{OS{Major: 11, Minor: 0, Patch: 0}, "11"},
	}
	for _, test := range tests {
		got := test.in.GetVersion()
		if got != test.exp {
			t.Errorf("Version mismatch: %s != %s", got, test.exp)
		}
	}
}
