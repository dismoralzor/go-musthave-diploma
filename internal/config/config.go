// Package config отвечает за разбор конфигурации сервиса gophermart из
// флагов командной строки и переменных окружения.
package config

import (
	"errors"
	"flag"
	"strings"
)

// Config содержит параметры запуска сервиса.
type Config struct {
	// RunAddress — адрес, на котором слушает HTTP-сервер.
	RunAddress string
	// DatabaseURI — строка подключения к PostgreSQL.
	DatabaseURI string
	// AccrualAddress — базовый URL системы расчёта начислений.
	AccrualAddress string
}

// ErrEmptyDatabaseURI возвращается, если не задан DSN базы данных: без неё
// сервис работать не может.
var ErrEmptyDatabaseURI = errors.New("database uri is required")

const (
	defaultRunAddress     = "localhost:8080"
	defaultDatabaseURI    = ""
	defaultAccrualAddress = ""
)

// Parse разбирает конфигурацию из аргументов командной строки args (без
// имени программы) и функции чтения переменных окружения getenv.
//
// Приоритет источников: переменная окружения > флаг > значение по умолчанию.
// Параметр getenv передаётся явно (а не берётся из os.Getenv напрямую),
// чтобы функцию можно было тестировать без реального окружения процесса.
func Parse(args []string, getenv func(string) string) (*Config, error) {
	fs := flag.NewFlagSet("gophermart", flag.ContinueOnError)

	runAddress := fs.String("a", defaultRunAddress, "адрес и порт запуска HTTP-сервера")
	databaseURI := fs.String("d", defaultDatabaseURI, "строка подключения к базе данных")
	accrualAddress := fs.String("r", defaultAccrualAddress, "адрес системы расчёта начислений")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	cfg := &Config{
		RunAddress:     *runAddress,
		DatabaseURI:    *databaseURI,
		AccrualAddress: *accrualAddress,
	}

	if v := getenv("RUN_ADDRESS"); v != "" {
		cfg.RunAddress = v
	}
	if v := getenv("DATABASE_URI"); v != "" {
		cfg.DatabaseURI = v
	}
	if v := getenv("ACCRUAL_SYSTEM_ADDRESS"); v != "" {
		cfg.AccrualAddress = v
	}

	cfg.AccrualAddress = normalizeAccrualAddress(cfg.AccrualAddress)

	if cfg.DatabaseURI == "" {
		return nil, ErrEmptyDatabaseURI
	}

	return cfg, nil
}

// normalizeAccrualAddress приводит адрес accrual к виду с явной схемой и
// без завершающего слэша. Пустая строка возвращается без изменений.
func normalizeAccrualAddress(addr string) string {
	if addr == "" {
		return addr
	}
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		addr = "http://" + addr
	}
	return strings.TrimRight(addr, "/")
}
