package neo

import (
	"context"
	"testing"

	lep "github.com/mgudov/logic-expression-parser"
	"github.com/metahub-registry/metahub-core/pkg/types"
	"github.com/metahub-registry/metahub-core/pkg/utils"
)

func TestExprSimple(t *testing.T) {
	ctx := context.Background()
	bs := utils.NewBootstrap()
	bs.SetYAMLPrefix("../../../misc/yaml")
	bs.SetEmptyAll()
	// TODO: Add ManifestData that matches annotation
	bs.AddBootstrap("annotation:foo:bar")
	bs.AddBootstrap("rules:rules/foo/bar.yaml")
	//bs.PostTestSleep = 120
	br, err := SetupTest(ctx, bs)
	_ = br
	if err != nil {
		t.Fatal(err)
	}
	defer db.TeardownTest(bs, br)
	// Actual test
	///// Eval all rules first
	err = db.RuleEvalAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	/// Now tests
	er := types.NewExprRes()
	query := `foo:bar=true`
	encQuery, err := utils.EncodeExpression(query)
	if err != nil {
		t.Fatal(err)
	}
	expr, err := lep.ParseExpression(encQuery)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.TraverseExpr(er, expr)
	if err != nil {
		t.Fatal(err)
	}

}
