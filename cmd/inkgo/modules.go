package main

import (
	"writer/ext"
	"writer/ext/export"
	"writer/ext/phone"
)

var modules = []ext.Module{phone.Module, export.Module}
