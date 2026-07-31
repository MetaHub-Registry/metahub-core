package types

import "testing"

func TestValidSlug(t *testing.T) {
	valid := []string{"qnib-solut", "web", "a1", "abc123", "a-b-c", "user"}
	for _, s := range valid {
		if !ValidSlug(s) {
			t.Errorf("ValidSlug(%q) = false, want true", s)
		}
	}
	invalid := []string{"", "-a", "a-", "A", "Web", "a_b", "a b", "a/b", "-"}
	for _, s := range invalid {
		if ValidSlug(s) {
			t.Errorf("ValidSlug(%q) = true, want false", s)
		}
	}
}

func TestValidNamespacePair(t *testing.T) {
	if !ValidNamespacePair("qnib-solut", "test") {
		t.Error("qnib-solut/test should be valid")
	}
	// reserved org "user" is rejected (collides with personal namespaces)
	if ValidNamespacePair(PersonalTeamSlug, "x") {
		t.Error("org=user must be rejected")
	}
	// malformed slugs rejected
	if ValidNamespacePair("Acme", "web") || ValidNamespacePair("acme", "We b") {
		t.Error("malformed slugs must be rejected")
	}
}
