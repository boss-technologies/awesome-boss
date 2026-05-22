package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/boss-technologies/awesome-boss/cmd/handlers"
)

// ---------- ВЕРСИЯ ----------
const version = "0.5.83"

// ---------- ОСНОВНАЯ ФУНКЦИЯ (диспетчер команд) ----------
func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "build":
		handlers.HandleBuild(os.Args[2:])
	case "generate":
		handlers.HandleGenerate(os.Args[2:])
	case "migrations":
		handlers.HandleMigrations(os.Args[2:])
	case "make":
		handlers.HandleMakeModels()
	case "new":
		handlers.HandleNew(os.Args[2:])
	case "run":
		handlers.HandleRun()
	case "version":
		handleVersion()
	default:
		fmt.Printf("❌ Неизвестная команда: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

// ---------- ВЫВОД СПРАВКИ ----------
func printUsage() {
	fmt.Println(`🐱 Awesome Boss CLI v0.7.0

Использование:
  boss <команда> [аргументы]

Доступные команды:
  build            Собрать проект в оптимизированный бинарник
  generate key     Сгенерировать секретный ключ для BAT
  migrations       Управление миграциями БД (Atlas)
    make <name>      Создать миграцию по изменениям в моделях
    use            Применить все новые миграции
    rollback         Откатить последнюю миграцию
  make             Сгенерировать Store для моделей (BossQ)
  new <name>       Создать новый проект
  run              Запустить сервер с горячей перезагрузкой
  version          Показать версию

Для справки по команде: boss <команда> -h
`)
}

// ---------- 7. КОМАНДА version ----------
func handleVersion() {
	fmt.Printf("Awesome Boss и Boss CLI версия: %s\n", version)
	fmt.Printf("Версия Go: %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

// Выполнено с любовью для Босса 🐈‍