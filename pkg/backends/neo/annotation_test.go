package neo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestAnnotationCRUD(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	bs := utils.NewBootstrap()
	bs.SetEmptyAll()
	//bs.PostTestSleep = 90
	br, err := SetupTest(ctx, bs)
	_ = br
	if err != nil {
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)
	// Actual test
	a0 := types.NewAnnotation("foo", "bar")
	a0_id, err := db.AnnotationGetInsert(ctx, a0)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotEqual(t, 0, a0_id)
	a0.ID = a0_id
	l0 := types.NewAnnotation("foo", "bar")
	l0_id, err := db.LabelGetInsert(ctx, l0)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotEqual(t, 0, l0_id)
	a, found, err := db.AnnotationGetByID(ctx, a0_id)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, found)
	assert.Equal(t, a0_id, a.ID)
	a0.Value = "baz"
	err = db.AnnotationUpdate(ctx, a0)
	if err != nil {
		t.Fatal(err)
	}
	a0.Key = "foz"
	err = db.AnnotationUpdate(ctx, a0)
	if err == nil {
		t.Fatal("expect key error")
	}
	a1 := types.NewAnnotation("foz", "bar")
	a1.ID = a0_id
	err = db.AnnotationUpdate(ctx, a0)
	if err == nil {
		t.Fatal("expect ID error")
	}

}
