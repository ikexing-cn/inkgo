package main

import (
	"writer/ext"
	"writer/ext/backup"
	"writer/ext/context"
	"writer/ext/export"
	"writer/ext/outline"
	"writer/ext/phone"
)

var backupModule, _ = backup.New(true)

var modules = []ext.Module{context.Module, outline.Module, backupModule, export.Module, phone.Module}
