package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRulesSatisfy(t *testing.T) {
	rlsEmpty := NewRules()
	assert.Equal(t, 0, len(rlsEmpty.Rules))
	rulesGlow150 := NewRules(
		RulePlatLinux,
	)
	assert.Equal(t, 1, len(rulesGlow150.Rules))
	assert.False(t, rulesGlow150.SatisfiedBy(rlsEmpty))
	assert.True(t, rlsEmpty.SatisfiedBy(rulesGlow150))
	assert.True(t, rulesGlow150.SatisfiedBy(rulesGlow150))

}
