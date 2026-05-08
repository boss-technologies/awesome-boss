package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/boss-technologies/awesome-boss/auth"
	"github.com/boss-technologies/awesome-boss/internal/bossq"
)

// ---------- ВЕРСИЯ ----------
const version = "0.5.0"

// ---------- ОСНОВНАЯ ФУНКЦИЯ (диспетчер команд) ----------
func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "build":
		handleBuild(os.Args[2:])
	case "generate":
		handleGenerate(os.Args[2:])
	case "new":
		handleNew(os.Args[2:])
	case "run":
		handleRun()
	case "version":
		handleVersion()
	case "make":
		handleMakeModels()
	default:
		fmt.Printf("❌ Неизвестная команда: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

// ---------- ВЫВОД СПРАВКИ ----------
func printUsage() {
	fmt.Println(`🐱 Awesome Boss CLI v0.5.0

Использование:
  boss <команда> [аргументы]

Доступные команды:
  build       Собрать проект в оптимизированный бинарник
  generate    Генерировать артефакты (ключи, модели, миграции)
  make        Сгенерировать код моделий с помощью BossQ
  new         Создать новый проект
  run         Запустить сервер с горячей перезагрузкой
  version     Показать версию

Для справки по команде: boss <команда> -h
`)
}

// ---------- 1. КОМАНДА build ----------
func handleBuild(args []string) {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	output := fs.String("o", "", "имя выходного бинарника")
	fs.Parse(args)

	// Определяем имя выходного файла
	outName := *output
	if outName == "" {
		// По умолчанию: имя текущей папки + .exe для Windows
		dir, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Ошибка: %v\n", err)
			os.Exit(1)
		}
		outName = filepath.Base(dir) + getExeSuffix()
	}

	// Собираем аргументы go build
	buildArgs := []string{
		"build",
		"-ldflags=-s -w",
		"-trimpath",
		"-o", outName,
	}

	cmd := exec.Command("go", buildArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Ошибка сборки: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Сборка завершена: %s\n", outName)
}

// ---------- 2. КОМАНДА generate (с подкомандами key, migration) ----------
func handleGenerate(args []string) {
	if len(args) == 0 {
		fmt.Println("❌ Укажите подкоманду: boss generate key|migration")
		os.Exit(1)
	}
	switch args[0] {
	case "key":
		handleGenerateKey(args[1:])
	case "migration":
		handleGenerateMigration(args[1:])
	default:
		fmt.Printf("❌ Неизвестная подкоманда: %s\n", args[0])
		os.Exit(1)
	}
}

func handleGenerateKey(args []string) {
	fs := flag.NewFlagSet("generate key", flag.ExitOnError)
	jsonFlag := fs.Bool("json", false, "вывод в JSON формате (опционально)")
	fs.Parse(args)

	hexKey, err := auth.GenerateSecretKey()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка генерации ключа: %v\n", err)
		os.Exit(1)
	}

	if *jsonFlag {
		// Простейший JSON вывод
		fmt.Printf(`{"key": "%s", "note": "Сохраните этот ключ в переменную BOSS_AUTH_SECRET"}`+"\n", hexKey)
	} else {
		fmt.Println(hexKey)
		fmt.Println("# Сохраните этот ключ в переменную окружения BOSS_AUTH_SECRET или в config.toml")
	}
}

func handleGenerateMigration(args []string) {
	if len(args) == 0 {
		fmt.Println("❌ Укажите название миграции: boss generate migration <имя>")
		os.Exit(1)
	}
	migrationName := args[0]

	// Папка для миграций
	migrationsDir := "migrations"

	// Создаём папку, если её ещё нет
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Не удалось создать директорию миграций: %v\n", err)
		os.Exit(1)
	}

	// Определяем следующий номер версии
	nextVersion := getNextMigrationVersion(migrationsDir)

	// Формируем имена файлов
	versionStr := fmt.Sprintf("%06d", nextVersion)
	upFile := filepath.Join(migrationsDir, fmt.Sprintf("%s_%s.up.sql", versionStr, migrationName))
	downFile := filepath.Join(migrationsDir, fmt.Sprintf("%s_%s.down.sql", versionStr, migrationName))

	// Шаблоны содержимого
	upContent := fmt.Sprintf("-- Миграция %s: %s (UP)\n", versionStr, migrationName)
	downContent := fmt.Sprintf("-- Миграция %s: %s (DOWN)\n", versionStr, migrationName)

	// Записываем файлы
	if err := os.WriteFile(upFile, []byte(upContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Ошибка создания файла %s: %v\n", upFile, err)
		os.Exit(1)
	}
	if err := os.WriteFile(downFile, []byte(downContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Ошибка создания файла %s: %v\n", downFile, err)
		os.Exit(1)
	}

	fmt.Printf("✅ Созданы миграции:\n")
	fmt.Printf("   📄 %s\n", upFile)
	fmt.Printf("   📄 %s\n", downFile)
}

// ---------- 3. КОМАНДА make ----------

func handleMakeModels() {
    // 1. Определяем рабочую директорию - откуда мы запускаем команду.
    dir, err := os.Getwd()
    if err != nil {
        fmt.Fprintf(os.Stderr, "❌ Не могу определить текущую папку: %v\n", err)
        os.Exit(1)
    }
    
    fmt.Printf("🔍 BossQ сканирует модели в '%s'...\n", dir)
    
    // 2. Вызываем генератор, передавая ему ТЕКУЩУЮ ДИРЕКТОРИЮ.
    if err := bossq.Generate(dir); err != nil {
        fmt.Fprintf(os.Stderr, "❌ Ошибка генерации: %v\n", err)
        os.Exit(1)
    }
    
    fmt.Println("✅ BossQ сгенерировал Store-файлы!")
}

// ---------- 4. КОМАНДА new ----------
func handleNew(args []string) {
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

	// Переходим в папку проекта и инициализируем модуль
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

	fmt.Printf("✅ Проект %s создан. Перейдите в папку и выполните 'boss run'\n", projectName)
	fmt.Println("/\\ /\\ Awesome Boss🐱")
}

// ---------- 5. КОМАНДА run ----------
func handleRun() {
	// Проверяем, есть ли main.go
	if _, err := os.Stat("main.go"); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "❌ Не найден main.go — убедитесь, что вы в папке проекта")
		os.Exit(1)
	}

	// Ищем air
	if airPath, err := exec.LookPath("air"); err == nil {
		fmt.Println("🚀🔄 Запуск с hot-reload (air)...")
		cmd := exec.Command(airPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Ошибка при запуске air: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Println("🚀 Запуск сервера (горячая перезагрузка не активна, установите air: go install github.com/cosmtrek/air@latest)")
		cmd := exec.Command("go", "run", ".")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Ошибка при запуске: %v\n", err)
			os.Exit(1)
		}
	}
}

// ---------- 6. КОМАНДА version ----------
func handleVersion() {
	fmt.Printf("Awesome Boss и Boss CLI версия: %s\n", version)
	fmt.Printf("Версия Go: %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

// ---------- ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ----------

func getExeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// createProjectStructure – копия из твоего new.go, почти без изменений
func createProjectStructure(projectPath, templateType string) error {
	// Создаём корневую папку
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		return err
	}

	// Создаём подпапки
	dirs := []string{
		filepath.Join(projectPath, "handlers"),
		filepath.Join(projectPath, "models"),
		filepath.Join(projectPath, "templates"),
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
	modelContent := `package models

// ExampleModel – пример модели. Замените на свои поля.
type ExampleModel struct {
    ID   int    ` + "`json:\"id\"`" + `
    Name string ` + "`json:\"name\"`" + `
}
`
	if templateType == "fintech" {
		modelContent = `package models

import "github.com/shopspring/decimal"

// ExampleModel – пример модели для FinTech.
type ExampleModel struct {
    ID      int             ` + "`json:\"id\"`" + `
    Amount  decimal.Decimal ` + "`json:\"amount\"`" + `
    Status  string          ` + "`json:\"status\"`" + `
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
`
os.WriteFile(filepath.Join(projectPath, ".env"), []byte(envContent), 0644)

// boss.toml
tomlContent := `mode = "` + templateType + `"

[server]
port = 8080
read_timeout = "30s"
write_timeout = "30s"
enable_gzip = true

[auth]
# secret_key будет взят из переменной окружения BOSS_AUTH_SECRET

[database]
host = "localhost"
port = 5432

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

	return nil
}

// generateMain – копия из твоего new.go
func generateMain(templateType, projectName string) string {
    if templateType == "fintech" {
        return `package main

import (
    "fmt"
    "log"
    "` + projectName + `/handlers"
    "github.com/boss-technologies/awesome-boss"
    "github.com/boss-technologies/awesome-boss/config"
    "github.com/boss-technologies/awesome-boss/internal/fintech"
)

func main() {
    cfg, err := config.LoadConfig("boss.toml")
    if err != nil {
        log.Fatalf("Ошибка загрузки конфигурации: %v", err)
    }

    app := awesomeboss.New(cfg) // режим fintech автоматом включит Audit

    // Защищаем все маршруты авторизацией BAT-токеном
    app.Use(fintech.RequireAuth(cfg.Auth.SecretKey))

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

// getNextMigrationVersion сканирует папку с миграциями и возвращает следующий номер.
// Имена файлов должны быть в формате "число_*.*", подходящем для golang-migrate.
func getNextMigrationVersion(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 1
	}

	maxVersion := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Ищем префикс до первого подчёркивания
		underscoreIndex := strings.Index(name, "_")
		if underscoreIndex <= 0 {
			continue
		}
		versionStr := name[:underscoreIndex]
		version, err := strconv.Atoi(versionStr)
		if err != nil {
			continue
		}
		if version > maxVersion {
			maxVersion = version
		}
	}
	return maxVersion + 1
}