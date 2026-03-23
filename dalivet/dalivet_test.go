package dalivet_test

import (
	"testing"

	"github.com/mibk/dali/dalivet"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, dalivet.Analyzer, "a")
}
