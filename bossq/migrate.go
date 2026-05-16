package bossq

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	internal "github.com/boss-technologies/awesome-boss/internal/bossq"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ColumnInfo содержит информацию о колонке таблицы в PostgreSQL.
type ColumnInfo struct {
	Name          string
	DataType      string
	UDTName       string
	IsNullable    bool
	ColumnDefault *string
}

// goTypeToPostgres преобразует имя типа Go в соответствующий тип PostgreSQL.
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

// LoadModels загружает все модели из указанной директории.
func LoadModels(dir string) ([]internal.ModelInfo, error) {
	return internal.ExtractModelsFromDir(dir)
}

// GetTableColumns возвращает список колонок таблицы из information_schema.
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

// GenerateCreateTable генерирует SQL CREATE TABLE на основе модели.
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
	if model.FTSLanguage != "" {
		cols = append(cols, "    search_vector tsvector")
	}
	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n%s\n);", model.TableName, strings.Join(cols, ",\n"))
}

// DiffModelWithDatabase сравнивает модель с реальными колонками в БД.
func DiffModelWithDatabase(model internal.ModelInfo, dbColumns []ColumnInfo) (upSQL, downSQL string) {
	dbColMap := make(map[string]ColumnInfo)
	for _, col := range dbColumns {
		dbColMap[col.Name] = col
	}

	var up, down []string

	for _, field := range model.Fields {
		expectedType := goTypeToPostgres(field.Type)
		if field.IsAutoinc {
			expectedType = "int8"
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
			// Изменение типа
			if !strings.EqualFold(dbCol.UDTName, expectedType) {
				newType := expectedType
				if field.IsAutoinc {
					newType = "BIGSERIAL"
				}
				up = append(up, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s USING %s::%s;",
					model.TableName, field.ColumnName, newType, field.ColumnName, newType))
				down = append(down, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s USING %s::%s;",
					model.TableName, field.ColumnName, dbCol.UDTName, field.ColumnName, dbCol.UDTName))
			}
			delete(dbColMap, field.ColumnName)
		}
	}

	// Удалённые колонки
	for colName, col := range dbColMap {
		up = append(up, fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", model.TableName, colName))
		down = append(down, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s;", model.TableName, colName, col.UDTName))
	}

	upSQL = strings.Join(up, "\n")
	downSQL = strings.Join(down, "\n")
	return
}

// WriteMigration создаёт файлы миграции с нумерацией.
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

// getNextMigrationVersion возвращает следующий номер версии на основе существующих файлов.
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

// validateDefault проверяет безопасность значения DEFAULT.
func validateDefault(val string) bool {
	if val == "NULL" || val == "TRUE" || val == "FALSE" || val == "true" || val == "false" {
		return true
	}
	if _, err := strconv.Atoi(val); err == nil {
		return true
	}
	if _, err := strconv.ParseFloat(val, 64); err == nil {
		return true
	}
	matched, _ := regexp.MatchString(`^'.*'$`, val)
	return matched
}

// MakeMigration сравнивает модели с БД и создаёт файлы миграции.
func MakeMigration(ctx context.Context, pool *pgxpool.Pool, modelsDir, migrationsDir string, name string) error {
	models, err := internal.ExtractModelsFromDir(modelsDir)
	if err != nil {
		return fmt.Errorf("ошибка загрузки моделей: %w", err)
	}
	if len(models) == 0 {
		return fmt.Errorf("не найдено моделей в директории %s", modelsDir)
	}

	var upSQL, downSQL strings.Builder
	for _, model := range models {
		exists, err := tableExists(ctx, pool, model.TableName)
		if err != nil {
			return fmt.Errorf("проверка существования таблицы %s: %w", model.TableName, err)
		}
		if !exists {
			upSQL.WriteString(GenerateCreateTable(model))
			upSQL.WriteString("\n\n")
			downSQL.WriteString(fmt.Sprintf("DROP TABLE IF EXISTS %s;\n\n", model.TableName))
			continue
		}
		dbCols, err := GetTableColumns(ctx, pool, model.TableName)
		if err != nil {
			return fmt.Errorf("получение колонок для %s: %w", model.TableName, err)
		}
		up, down := DiffModelWithDatabase(model, dbCols)
		if up != "" {
			upSQL.WriteString(up)
			upSQL.WriteString("\n\n")
			downSQL.WriteString(down)
			downSQL.WriteString("\n\n")
		}
	}

	if upSQL.Len() == 0 {
		log.Println("✨ Нет изменений в моделях. Миграция не требуется.")
		return nil
	}

	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		return fmt.Errorf("не удалось создать папку %s: %w", migrationsDir, err)
	}

	timestamp := time.Now().Format("20060102150405")
	migrationName := timestamp + "_" + name
	if name == "" {
		migrationName = timestamp + "_auto"
	}
	upPath := filepath.Join(migrationsDir, migrationName+".up.sql")
	downPath := filepath.Join(migrationsDir, migrationName+".down.sql")

	if err := os.WriteFile(upPath, []byte(upSQL.String()), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(downPath, []byte(downSQL.String()), 0644); err != nil {
		return err
	}

	log.Printf("✅ Создана миграция %s\n   📄 %s\n   📄 %s", migrationName, upPath, downPath)
	return nil
}

// ApplyMigrations применяет все неприменённые миграции с проверкой целостности.
func ApplyMigrations(ctx context.Context, pool *pgxpool.Pool, migrationsDir string) error {
	// Блокировка от параллельного выполнения
	lockID := int64(1234567890)
	_, err := pool.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", lockID)
	if err != nil {
		return fmt.Errorf("не удалось захватить advisory lock: %w", err)
	}

	if err := ensureMigrationsTable(ctx, pool); err != nil {
		return err
	}

	applied, err := getAppliedMigrationsWithChecksums(ctx, pool)
	if err != nil {
		return err
	}

	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		log.Println("📭 Нет миграций в папке", migrationsDir)
		return nil
	}
	sort.Strings(files)

	for _, file := range files {
		base := strings.TrimSuffix(filepath.Base(file), ".up.sql")
		record, exists := applied[base]

		// Вычисляем хеш текущего файла
		hash, err := fileChecksum(file)
		if err != nil {
			return fmt.Errorf("ошибка вычисления хеша %s: %w", file, err)
		}

		if exists {
			// Проверяем, что хеш не изменился
			if record.Checksum != hash {
				return fmt.Errorf("миграция %s была изменена после применения! (ожидался %s, получен %s)",
					base, record.Checksum, hash)
			}
			continue // уже применена и не изменилась
		}

		// Новая миграция – применяем
		log.Printf("⚡ Применяю миграцию %s...", base)
		if err := applyMigrationFile(ctx, pool, file, hash); err != nil {
			return fmt.Errorf("ошибка в %s: %w", file, err)
		}
		log.Printf("   ✅ %s применена", base)
	}
	log.Println("🎉 Все миграции успешно применены.")
	return nil
}

// RollbackLastMigration откатывает последнюю миграцию.
func RollbackLastMigration(ctx context.Context, pool *pgxpool.Pool, migrationsDir string) error {
	lockID := int64(1234567890)
	_, err := pool.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", lockID)
	if err != nil {
		return fmt.Errorf("не удалось захватить advisory lock: %w", err)
	}

	if err := ensureMigrationsTable(ctx, pool); err != nil {
		return err
	}

	var lastVersion string
	err = pool.QueryRow(ctx, "SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1").Scan(&lastVersion)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return fmt.Errorf("нет применённых миграций для отката")
		}
		return err
	}
	downPath := filepath.Join(migrationsDir, lastVersion+".down.sql")
	if _, err := os.Stat(downPath); os.IsNotExist(err) {
		return fmt.Errorf("файл отката %s не найден", downPath)
	}
	log.Printf("🔄 Откатываю миграцию %s...", lastVersion)
	// При откате не проверяем хеш, но выполняем в транзакции
	if err := applyRawSQLFile(ctx, pool, downPath); err != nil {
		return fmt.Errorf("ошибка отката: %w", err)
	}
	_, err = pool.Exec(ctx, "DELETE FROM schema_migrations WHERE version = $1", lastVersion)
	if err != nil {
		return fmt.Errorf("не удалось удалить версию %s: %w", lastVersion, err)
	}
	log.Printf("   ✅ %s откачена", lastVersion)
	return nil
}

// ------------------------------------------------------------
// Внутренние вспомогательные функции
// ------------------------------------------------------------

// migrationRecord хранит информацию о применённой миграции.
type migrationRecord struct {
	Version  string
	Checksum string
}

// ensureMigrationsTable создаёт таблицу schema_migrations с колонкой checksum.
func ensureMigrationsTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			checksum TEXT NOT NULL,
			applied_at TIMESTAMPTZ DEFAULT NOW()
		)
	`)
	return err
}

// getAppliedMigrationsWithChecksums возвращает карту версия → запись.
func getAppliedMigrationsWithChecksums(ctx context.Context, pool *pgxpool.Pool) (map[string]migrationRecord, error) {
	rows, err := pool.Query(ctx, "SELECT version, checksum FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	applied := make(map[string]migrationRecord)
	for rows.Next() {
		var rec migrationRecord
		if err := rows.Scan(&rec.Version, &rec.Checksum); err != nil {
			return nil, err
		}
		applied[rec.Version] = rec
	}
	return applied, rows.Err()
}

// tableExists проверяет существование таблицы.
func tableExists(ctx context.Context, pool *pgxpool.Pool, tableName string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_name = $1
		)`, tableName).Scan(&exists)
	return exists, err
}

// fileChecksum вычисляет SHA256 хеш содержимого файла.
func fileChecksum(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// applyMigrationFile выполняет up-скрипт и сохраняет хеш.
func applyMigrationFile(ctx context.Context, pool *pgxpool.Pool, filePath, checksum string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Выполняем весь скрипт одной командой (pgx умеет несколько ";" в одном вызове)
	sqlBytes, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, string(sqlBytes))
	if err != nil {
		return fmt.Errorf("ошибка выполнения SQL: %w", err)
	}

	// Сохраняем запись о применении
	base := strings.TrimSuffix(filepath.Base(filePath), ".up.sql")
	_, err = tx.Exec(ctx, "INSERT INTO schema_migrations (version, checksum) VALUES ($1, $2)", base, checksum)
	if err != nil {
		return fmt.Errorf("не удалось записать версию %s: %w", base, err)
	}

	return tx.Commit(ctx)
}

// applyRawSQLFile выполняет SQL-файл без проверки хеша (для rollback).
func applyRawSQLFile(ctx context.Context, pool *pgxpool.Pool, filePath string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	sqlBytes, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, string(sqlBytes))
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Выполнено с любовью для Босса 🐈