package bossq

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	internal "github.com/boss-technologies/awesome-boss/internal/bossq"

	"github.com/golang-migrate/migrate/v4"
	pgxv5 "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
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

// ColumnInfo представляет колонку таблицы в базе данных
type ColumnInfo struct {
	Name          string
	DataType      string
	UDTName       string
	IsNullable    bool
	ColumnDefault *string
}

// goTypeToPostgres преобразует имя типа Go в тип PostgreSQL (UDT name)
func goTypeToPostgres(goType string) string {
	switch goType {
	case "int", "int32":
		return "int4"
	case "int64":
		return "int8"
	case "uint", "uint32":
		return "int4"
	case "uint64":
		return "int8"
	case "string":
		return "text"
	case "bool":
		return "bool"
	case "float32":
		return "float4"
	case "float64":
		return "float8"
	case "time.Time":
		return "timestamptz"
	case "decimal.Decimal":
		return "numeric"
	case "[]byte":
		return "bytea"
	case "json.RawMessage":
		return "jsonb"
	default:
		return "text"
	}
}

// LoadModels загружает все модели из указанной директории и возвращает срез ModelInfo
func LoadModels(dir string) ([]internal.ModelInfo, error) {
	return internal.ExtractModelsFromDir(dir)
}

// GetTableColumns возвращает список колонок таблицы из information_schema.
// Использует LOWER(table_name) для регистронезависимого сравнения.
func GetTableColumns(ctx context.Context, pool *pgxpool.Pool, tableName string) ([]ColumnInfo, error) {
	query := `
		SELECT column_name, data_type, udt_name, is_nullable, column_default
		FROM information_schema.columns
		WHERE LOWER(table_name) = LOWER($1)
		ORDER BY ordinal_position
	`
	rows, err := pool.Query(ctx, query, tableName)
	if err != nil {
		return nil, fmt.Errorf("query information_schema for %s: %w", tableName, err)
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		var nullable string
		err := rows.Scan(&col.Name, &col.DataType, &col.UDTName, &nullable, &col.ColumnDefault)
		if err != nil {
			return nil, fmt.Errorf("scan column info: %w", err)
		}
		col.IsNullable = nullable == "YES"
		columns = append(columns, col)
	}
	return columns, rows.Err()
}

// GenerateCreateTable создаёт SQL для CREATE TABLE на основе модели.
// Безопасность DEFAULT гарантируется вызовом validateDefault.
func GenerateCreateTable(model internal.ModelInfo) string {
	var cols []string
	for _, f := range model.Fields {
		sqlType := goTypeToPostgres(f.Type)
		if f.IsAutoinc {
			sqlType = "BIGSERIAL"
		}
		colDef := fmt.Sprintf("%s %s", f.ColumnName, sqlType)
		if f.IsPK {
			colDef += " PRIMARY KEY"
		}
		if f.IsNotNull {
			colDef += " NOT NULL"
		}
		if f.HasDefault && validateDefault(f.DefaultVal) {
			colDef += fmt.Sprintf(" DEFAULT %s", f.DefaultVal)
		} else if f.HasDefault {
			log.Printf("⚠️ Небезопасное значение DEFAULT для %s.%s пропущено", model.TableName, f.ColumnName)
		}
		cols = append(cols, "    "+colDef)
	}
	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n%s\n);", model.TableName, strings.Join(cols, ",\n"))
}

// DiffModelWithDatabase сравнивает модель с реальной таблицей и возвращает SQL для UP и DOWN миграций.
func DiffModelWithDatabase(model internal.ModelInfo, dbColumns []ColumnInfo) (upSQL, downSQL string) {
	dbColMap := make(map[string]ColumnInfo)
	for _, col := range dbColumns {
		dbColMap[col.Name] = col
	}

	var up, down []string

	for _, field := range model.Fields {
		expectedType := goTypeToPostgres(field.Type)
		if field.IsAutoinc {
			expectedType = "int8" // автоинкремент базируется на bigint
		}
		dbCol, exists := dbColMap[field.ColumnName]

		if !exists {
			// Новое поле
			colDef := fmt.Sprintf("%s %s", field.ColumnName, expectedType)
			if field.IsAutoinc {
				colDef = fmt.Sprintf("%s BIGSERIAL", field.ColumnName)
			}
			if field.IsNotNull {
				colDef += " NOT NULL"
			}
			if field.HasDefault && validateDefault(field.DefaultVal) {
				colDef += fmt.Sprintf(" DEFAULT %s", field.DefaultVal)
			}
			up = append(up, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s;", model.TableName, colDef))
			down = append(down, fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", model.TableName, field.ColumnName))
		} else {
			// Проверка изменения типа
			if !strings.EqualFold(dbCol.UDTName, expectedType) {
				newType := expectedType
				if field.IsAutoinc {
					newType = "BIGSERIAL"
				}
				// Критическое исправление: добавляем USING с приведением
				up = append(up, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s USING %s::%s;",
					model.TableName, field.ColumnName, newType, field.ColumnName, newType))
				down = append(down, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s USING %s::%s;",
					model.TableName, field.ColumnName, dbCol.UDTName, field.ColumnName, dbCol.UDTName))
			}
			delete(dbColMap, field.ColumnName)
		}
	}

	// Оставшиеся колонки из БД — это удалённые поля
	for colName, col := range dbColMap {
		up = append(up, fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", model.TableName, colName))
		down = append(down, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s;", model.TableName, colName, col.UDTName))
	}

	upSQL = strings.Join(up, "\n")
	downSQL = strings.Join(down, "\n")
	return
}

// WriteMigration записывает UP и DOWN SQL в файлы миграций.
func WriteMigration(tableName, migrationType, upSQL, downSQL string) error {
	migrationsDir := "migrations"
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		return err
	}
	nextVer := getNextMigrationVersion(migrationsDir)
	versionStr := fmt.Sprintf("%06d", nextVer)
	name := fmt.Sprintf("%s_%s_%s", versionStr, migrationType, tableName)

	upPath := filepath.Join(migrationsDir, name+".up.sql")
	downPath := filepath.Join(migrationsDir, name+".down.sql")

	if err := os.WriteFile(upPath, []byte(upSQL+"\n"), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(downPath, []byte(downSQL+"\n"), 0644); err != nil {
		return err
	}
	fmt.Printf("   📄 %s\n", upPath)
	fmt.Printf("   📄 %s\n", downPath)
	return nil
}

// getNextMigrationVersion возвращает следующий номер версии миграции.
func getNextMigrationVersion(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 1
	}
	max := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		parts := strings.SplitN(e.Name(), "_", 2)
		if len(parts) < 2 {
			continue
		}
		if v, err := strconv.Atoi(parts[0]); err == nil && v > max {
			max = v
		}
	}
	return max + 1
}

// validateDefault проверяет, что значение DEFAULT безопасно для вставки в SQL.
// Разрешены только литералы чисел, строки в одинарных кавычках и NULL.
func validateDefault(val string) bool {
	// Простейшая проверка: число, 'строка', NULL, TRUE/FALSE
	if val == "NULL" || val == "TRUE" || val == "FALSE" || val == "true" || val == "false" {
		return true
	}
	// Проверка на число
	if _, err := strconv.Atoi(val); err == nil {
		return true
	}
	if _, err := strconv.ParseFloat(val, 64); err == nil {
		return true
	}
	// Строка в одинарных кавычках
	matched, _ := regexp.MatchString(`^'.*'$`, val)
	return matched
}

// Выполнено с любовью для Босса 🐈‍