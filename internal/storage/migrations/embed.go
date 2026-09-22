// Package migrations встраивает SQL-файлы миграций схемы базы данных в
// бинарный файл сервиса с помощью go:embed.
package migrations

import "embed"

// FS содержит встроенные файлы миграций.
//
//go:embed *.sql
var FS embed.FS
