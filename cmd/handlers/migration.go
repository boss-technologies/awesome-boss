package handlers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

// HandleMigrations – точка входа для команд миграций
func HandleMigrations(args []string) {
	if len(args) == 0 {
		fmt.Println("❌ Укажите подкоманду: boss migrations [diff|apply|rollback]")
		os.Exit(1)
	}

	switch args[0] {
	case "make":
		name := "auto"
		if len(args) > 1 {
			name = args[1]
		}
		atlasDiff(name)
	case "use":
		atlasApply()
	case "rollback":
		atlasRollback()
	default:
		fmt.Printf("❌ Неизвестная подкоманда: %s\n", args[0])
		os.Exit(1)
	}
}

// atlasDiff запускает atlas migrate diff через Docker
func atlasDiff(name string) {
	checkAtlasDocker()
	wd := getCwd()
	cmd := exec.Command(
		"docker", "run", "--rm",
		"-v", wd+":/project",
		"-w", "/project",
		"arigaio/atlas",
		"migrate", "diff", "--env", "bossq", name,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Ошибка создания миграции: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ Миграция создана в папке migrations/")
}

// atlasApply применяет неприменённые миграции через Docker
func atlasApply() {
	checkAtlasDocker()
	_ = godotenv.Load()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		fmt.Fprintln(os.Stderr, "❌ Переменная окружения DATABASE_URL не установлена")
		os.Exit(1)
	}
	wd := getCwd()
	cmd := exec.Command(
		"docker", "run", "--rm",
		"-v", wd+":/project",
		"-w", "/project",
		"arigaio/atlas",
		"migrate", "apply", "--env", "bossq", "--url", databaseURL,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Ошибка применения миграций: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("🎉 Все миграции успешно применены.")
}

// atlasRollback откатывает последнюю миграцию (выполняет down.sql)
func atlasRollback() {
	checkAtlasDocker()
	_ = godotenv.Load()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		fmt.Fprintln(os.Stderr, "❌ Переменная окружения DATABASE_URL не установлена")
		os.Exit(1)
	}
	wd := getCwd()
	files, err := filepath.Glob(filepath.Join(wd, "migrations", "*.up.sql"))
	if err != nil || len(files) == 0 {
		fmt.Fprintln(os.Stderr, "❌ Нет миграций для отката")
		os.Exit(1)
	}
	last := files[len(files)-1]
	base := strings.TrimSuffix(filepath.Base(last), ".up.sql")
	downFile := filepath.Join(wd, "migrations", base+".down.sql")
	if _, err := os.Stat(downFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "❌ Файл отката %s не найден\n", downFile)
		os.Exit(1)
	}
	// Выполняем down.sql через psql в контейнере
	cmd := exec.Command(
		"docker", "run", "--rm",
		"-v", wd+":/project",
		"-w", "/project",
		"postgres:15",
		"psql", databaseURL, "-f", "/project/"+downFile,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Ошибка отката: %v\n", err)
		os.Exit(1)
	}
	// Удаляем запись из schema_migrations
	delCmd := exec.Command(
		"docker", "run", "--rm",
		"postgres:15",
		"psql", databaseURL, "-c", fmt.Sprintf("DELETE FROM schema_migrations WHERE version='%s';", base),
	)
	delCmd.Stdout = os.Stdout
	delCmd.Stderr = os.Stderr
	_ = delCmd.Run() // не критично, если не удалится
	fmt.Printf("✅ Миграция %s откачена\n", base)
}

// checkAtlasDocker проверяет наличие образа arigaio/atlas, при необходимости скачивает
func checkAtlasDocker() {
	cmd := exec.Command("docker", "image", "inspect", "arigaio/atlas")
	if err := cmd.Run(); err != nil {
		fmt.Println("📦 Образ arigaio/atlas не найден, загружаю...")
		pull := exec.Command("docker", "pull", "arigaio/atlas")
		pull.Stdout = os.Stdout
		pull.Stderr = os.Stderr
		if err := pull.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Не удалось загрузить образ atlas: %v\n", err)
			os.Exit(1)
		}
	}
}

// getCwd возвращает текущую рабочую директорию
func getCwd() string {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка получения рабочей директории: %v\n", err)
		os.Exit(1)
	}
	return wd
}

// Выполнено с любовью для Босса 🐈‍