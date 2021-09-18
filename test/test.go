package test

import (
	. "github.com/black-desk/go-method-finder/test/pkg1"
	"github.com/black-desk/go-method-finder/test/pkg2"
)

type TARGET struct {
	BASE
	pkg2.BASE2
}

func (t *TARGET) FUNC3() {}
