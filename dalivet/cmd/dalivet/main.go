package main

import (
	"github.com/mibk/dali/dalivet"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(dalivet.Analyzer)
}
