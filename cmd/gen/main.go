package main

import (
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	"gorm.io/gen"
)

func main() {
	g := gen.NewGenerator(gen.Config{OutPath: "internal/dal/query", Mode: gen.WithQueryInterface, UseAny: true})
	g.ApplyBasic(model.Workspace{})
	g.Execute()
}
