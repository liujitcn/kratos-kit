package driver

import (
	"gorm.io/gorm"
)

var Opens = map[string]func(string) gorm.Dialector{}
