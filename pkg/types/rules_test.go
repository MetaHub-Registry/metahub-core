package types

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRulesSatisfy(t *testing.T) {
	ctx := context.Background()
	rlsEmpty := NewRules()
	assert.Equal(t, 0, len(rlsEmpty.Rules))
	rulesGlow150 := NewRules(
		RulePlatLinux,
	)
	assert.Equal(t, 1, len(rulesGlow150.Rules))
	assert.False(t, rulesGlow150.SatisfiedBy(ctx, rlsEmpty))
	assert.True(t, rlsEmpty.SatisfiedBy(ctx, rulesGlow150))
	assert.True(t, rulesGlow150.SatisfiedBy(ctx, rulesGlow150))

}

func TestRulesSatisfyArch(t *testing.T) {
	ctx := context.Background()
	rlsEmpty := NewRules()
	assert.Equal(t, 0, len(rlsEmpty.Rules))
	rulesGlow150 := NewRules(
		RulePlatArm64,
	)
	amdode := NewRules(
		RulePlatAmd64,
	)
	assert.Equal(t, 1, len(rulesGlow150.Rules))
	assert.False(t, rulesGlow150.SatisfiedBy(ctx, rlsEmpty))
	assert.True(t, rlsEmpty.SatisfiedBy(ctx, rulesGlow150))
	assert.False(t, rulesGlow150.SatisfiedBy(ctx, amdode))

}
