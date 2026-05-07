package bossq

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	pgxv5 "github.com/golang-migrate/migrate/v4/database/pgx/v5" // алиас pgxv5
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// RunMigrations применяет все ожидающие миграции из указанной директории.
func RunMigrations(connString, migrationsDir string) error {
	// 1. Создаем подключение через database/sql, используя драйвер pgx.
	db, err := sql.Open("pgx", connString)
	if err != nil {
		return fmt.Errorf("bossq: не удалось подключиться к БД: %w", err)
	}
	defer db.Close()

	// 2. Инициализируем драйвер golang-migrate для работы с pgx.
	driver, err := pgxv5.WithInstance(db, &pgxv5.Config{})
	if err != nil {
		return fmt.Errorf("bossq: ошибка инициализации драйвера pgx: %w", err)
	}

	// 3. Создаем экземпляр мигратора, указав источник файлов и драйвер БД.
	m, err := migrate.NewWithDatabaseInstance(
		"file://"+migrationsDir,
		"pgx",
		driver,
	)
	if err != nil {
		return fmt.Errorf("bossq: ошибка создания экземпляра migrate: %w", err)
	}

	// 4. Запускаем миграции.
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("bossq: ошибка применения миграций: %w", err)
	}

	log.Println("✅ Миграции успешно применены")
	return nil
}

// Выполнено с любовью для Босса 🐈‍