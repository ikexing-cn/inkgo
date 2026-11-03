package main

import (
	"writer/ext"
	"writer/ext/backup"
	"writer/ext/export"
	"writer/ext/phone"
)

var backupModule, _ = backup.New(true)

var modules = []ext.Module{phone.Module, export.Module, backupModule}
