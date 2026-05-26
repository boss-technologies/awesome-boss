package handlers

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ---------- 5. КОМАНДА new ----------
func HandleNew(args []string) {
	fs := flag.NewFlagSet("new", flag.ExitOnError)
	template := fs.String("t", "normal", "Шаблон проекта (normal, fintech)")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Println("❌ Укажите имя проекта: boss new <имя> [-t шаблон]")
		os.Exit(1)
	}
	projectName := fs.Arg(0)

	if err := createProjectStructure(projectName, *template); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка создания структуры: %v\n", err)
		os.Exit(1)
	}

	wd, _ := os.Getwd()
	projectPath := filepath.Join(wd, projectName)
	if err := os.Chdir(projectPath); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка: %v\n", err)
		os.Exit(1)
	}
	defer os.Chdir(wd)

	// go mod init
	if err := exec.Command("go", "mod", "init", projectName).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка go mod init: %v\n", err)
		os.Exit(1)
	}
	// go get awesome-boss
	if err := exec.Command("go", "get", "github.com/boss-technologies/awesome-boss@latest").Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка добавления зависимости: %v\n", err)
		os.Exit(1)
	}
	if err := exec.Command("go", "mod", "tidy").Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка go mod tidy: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Проект %s создан. Перейдите в папку и выполните 'boss run'\n", projectName)
	fmt.Println("/\\ /\\ Awesome Boss🐱")
}

// ---------- ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ДЛЯ NEW ----------
func createProjectStructure(projectPath, templateType string) error {
	// Создаём корневую папку
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		return err
	}
	dirs := []string{
		filepath.Join(projectPath, "handlers"),
		filepath.Join(projectPath, "models"),
		filepath.Join(projectPath, "templates"),
		filepath.Join(projectPath, "migrations"),
		filepath.Join(projectPath, "atlas"),
	}
	if templateType == "fintech" {
		dirs = append(dirs,
			filepath.Join(projectPath, "middleware"),
			filepath.Join(projectPath, "config"),
		)
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	// main.go
	mainContent := generateMain(templateType, projectPath)
	if err := os.WriteFile(filepath.Join(projectPath, "main.go"), []byte(mainContent), 0644); err != nil {
		return err
	}

	// Пример модели
	modelContent := `
// ExampleModel – пример модели. Замените на свои поля.
// bossq:table=examples
type ExampleModel struct {
    ID   int    ` + "`bossq:\"pk\"`" + `
    Name string ` + "`bossq:\"unique\"`" + `
}
`
	if templateType == "fintech" {
		modelContent = `package models

import "github.com/boss-technologies/awesome-boss/decimal"

// bossq:table=examples
type ExampleModel struct {
    ID      int             ` + "`bossq:\"pk,autoinc,column=id\"` " + `
    Amount  decimal.Decimal ` + "`bossq:\"column=amount\"`" + `
    Status  string          ` + "`bossq:\"column=status\"`" + `
}
`
	}
	if err := os.WriteFile(filepath.Join(projectPath, "models", "example.go"), []byte(modelContent), 0644); err != nil {
		return err
	}

	// Пример обработчика
	handlerContent := `package handlers

import (
    "github.com/boss-technologies/awesome-boss/core"
)

// ExampleHandler – пример обработчика.
func ExampleHandler(ctx *core.BossContext) error {
    return ctx.JSON(200, map[string]string{"message": "Hello from Awesome Boss!"})
}
`
	if err := os.WriteFile(filepath.Join(projectPath, "handlers", "example.go"), []byte(handlerContent), 0644); err != nil {
		return err
	}

	// Шаблон index.html
	htmlContent := `<!DOCTYPE html>
<html>
<head>
    <title>Awesome Boss</title>
</head>
<body>
    <h1>Добро пожаловать, разработчик!</h1>
    <p>Эта страница была сделана на Fur</p>
</body>
</html>`
	if err := os.WriteFile(filepath.Join(projectPath, "templates", "index.html"), []byte(htmlContent), 0644); err != nil {
		return err
	}

	// Для fintech – middleware и config
	if templateType == "fintech" {
		auditContent := `package middleware

import (
    "log/slog"
    "time"
    "github.com/boss-technologies/awesome-boss"
)

func Audit(next awesomeboss.Handler) awesomeboss.Handler {
    return func(ctx *awesomeboss.BossContext) error {
        start := time.Now()
        err := next(ctx)
        slog.Info("request",
            "method", ctx.Method(),
            "path", ctx.Path(),
            "status", ctx.Response.StatusCode(),
            "duration_ms", time.Since(start).Milliseconds(),
        )
        return err
    }
}
`
		if err := os.WriteFile(filepath.Join(projectPath, "middleware", "audit.go"), []byte(auditContent), 0644); err != nil {
			return err
		}
	}

	// .gitignore
	gitignoreContent := `# Binaries
*.exe
*.exe~
*.dll
*.so
*.dylib

.env

# Test binary
*.test

# Output of the go coverage tool
*.out

# Dependency directories
vendor/

# Go workspace file
go.work
go.work.sum

# IDE
.idea/
.vscode/
*.swp
*.swo

# OS
.DS_Store
`
	if err := os.WriteFile(filepath.Join(projectPath, ".gitignore"), []byte(gitignoreContent), 0644); err != nil {
		return err
	}

	// .env
	envContent := `# Секретный ключ для BAT (сгенерируйте командой 'boss generate key')
BOSS_AUTH_SECRET=
DATABASE_URL=
`
	os.WriteFile(filepath.Join(projectPath, ".env"), []byte(envContent), 0644)

	// boss.toml
	tomlContent := `mode = "` + templateType + `"

[server]
port = 8080
enable_gzip = true

[auth]
# secret_key будет взят из переменной окружения BOSS_AUTH_SECRET

[database]
# тоже будет взят из переменной окружения DATABASE_URL

[logging]
level = "debug"
`
	if err := os.WriteFile(filepath.Join(projectPath, "boss.toml"), []byte(tomlContent), 0644); err != nil {
		return err
	}

	// README.md
	readmeContent := "# " + filepath.Base(projectPath) + `

Проект создан с помощью **Awesome Boss** – FinTech-фреймворка для Go.

## Структура

- ` + "`handlers/`" + ` – обработчики запросов
- ` + "`models/`" + ` – модели данных
- ` + "`templates/`" + ` – HTML-шаблоны (Fur)
- ` + "`migrations/`" + ` – миграции БД (управляются Atlas)
- ` + "`atlas/`" + ` – конфигурация Atlas (loader.go)
`
	if templateType == "fintech" {
		readmeContent += "- `middleware/` – дополнительная прослойка (аудит, аутентификация)\n- `config/` – конфигурация\n"
	}
	readmeContent += `
## Запуск

` + "```bash\nboss run\n```\n"

	if err := os.WriteFile(filepath.Join(projectPath, "README.md"), []byte(readmeContent), 0644); err != nil {
		return err
	}

	// ------------------- Atlas files -------------------
	atlasHcl := `data "external_schema" "bossq" {
    program = [
        "go",
        "run",
        "./atlas/gentool.go",
    ]
}

env "bossq" {
    src = data.external_schema.bossq.url
    dev = "docker://postgres/15/dev?search_path=public"
    migration {
        dir = "file://migrations"
    }
    format {
        migrate {
            diff = "{{ sql . \"  \" }}"
        }
    }
}
`
	if err := os.WriteFile(filepath.Join(projectPath, "atlas.hcl"), []byte(atlasHcl), 0644); err != nil {
		return err
	}

	gentoolContent := `// atlas/gentool.go
package main

import (
    "fmt"
    "log"
    "strings"

    "github.com/boss-technologies/awesome-boss/bossq"
)

func main() {
    models, err := bossq.ExtractModelsFromDir("./models")
    if err != nil {
        log.Fatalf("Ошибка загрузки моделей: %v", err)
    }

    var sql strings.Builder
    for _, model := range models {
        sql.WriteString(buildCreateTable(model))
        sql.WriteString("\n\n")
        if model.FTSLanguage != "" {
            sql.WriteString(buildFTSColumnsAndIndexes(model))
            sql.WriteString("\n\n")
        }
    }
    fmt.Print(sql.String())
}

func buildCreateTable(model bossq.ModelInfo) string {
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
        cols = append(cols, "    "+colDef)
    }
    return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n%s\n);", model.TableName, strings.Join(cols, ",\n"))
}

func buildFTSColumnsAndIndexes(model bossq.ModelInfo) string {
    table := model.TableName
    lang := model.FTSLanguage

    addColumn := fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS search_vector tsvector;", table)
    createIndex := fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_search ON %s USING GIN (search_vector);", table, table)

    var weightedFields []string
    for _, f := range model.FTSFields {
        weight := f.Weight
        if weight == "" {
            weight = "D"
        }
        weightedFields = append(weightedFields, fmt.Sprintf("setweight(to_tsvector('%s', COALESCE(%s, '')), '%s')", lang, f.ColumnName, weight))
    }
    tsvectorExpr := strings.Join(weightedFields, " || ")
    if tsvectorExpr == "" {
        tsvectorExpr = fmt.Sprintf("to_tsvector('%s', COALESCE(title, '') || ' ' || COALESCE(content, ''))", lang)
    }

    functionSQL := fmt.Sprintf("CREATE OR REPLACE FUNCTION update_%s_search_vector() RETURNS trigger AS %s\nBEGIN\n    NEW.search_vector := %s;\n    RETURN NEW;\nEND;\n%s LANGUAGE plpgsql;", table, "$function$", tsvectorExpr, "$function$")

    triggerSQL := fmt.Sprintf("DROP TRIGGER IF EXISTS trigger_update_%s_search_vector ON %s;\nCREATE TRIGGER trigger_update_%s_search_vector\n    BEFORE INSERT OR UPDATE ON %s\n    FOR EACH ROW EXECUTE FUNCTION update_%s_search_vector();", table, table, table, table, table)

    return addColumn + "\n\n" + createIndex + "\n\n" + functionSQL + "\n\n" + triggerSQL
}

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
`
	if err := os.WriteFile(filepath.Join(projectPath, "atlas", "gentool.go"), []byte(gentoolContent), 0644); err != nil {
		return err
	}
	return nil
}

func generateMain(templateType, projectName string) string {
	if templateType == "fintech" {
		return `package main

import (
    "fmt"
    "log"
    "` + projectName + `/handlers"
    "github.com/boss-technologies/awesome-boss"
    "github.com/boss-technologies/awesome-boss/config"
)

func main() {
    cfg, err := config.LoadConfig("boss.toml")
    if err != nil {
        log.Fatalf("Ошибка загрузки конфигурации: %v", err)
    }

    app := awesomeboss.New(cfg) // режим fintech автоматом включит Audit

    app.Get("/", handlers.ExampleHandler)

    addr := fmt.Sprintf(":%d", cfg.Server.Port)
    log.Fatal(app.Run(addr))
}
`
	}
	// normal
	return `package main

import (
    "fmt"
    "log"
    "` + projectName + `/handlers"
    "github.com/boss-technologies/awesome-boss"
    "github.com/boss-technologies/awesome-boss/config"
)

func main() {
    cfg, err := config.LoadConfig("boss.toml")
    if err != nil {
        log.Fatalf("Ошибка загрузки конфигурации: %v", err)
    }

    app := awesomeboss.New(cfg)

    app.Get("/", handlers.ExampleHandler)
    addr := fmt.Sprintf(":%d", cfg.Server.Port)
    log.Fatal(app.Run(addr))
}
`
}

// Выполнено с любовью для Босса 🐈‍