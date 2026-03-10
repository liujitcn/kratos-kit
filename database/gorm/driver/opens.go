package driver

import (
	g "gorm.io/gorm"
)

var Opens = map[string]func(string) g.Dialector{}
