package handlers

import (
	"fmt"
	"os"
	"os/exec"
)

// ---------- 6. КОМАНДА run ----------
func HandleRun() {
	if _, err := os.Stat("main.go"); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "❌ Не найден main.go — убедитесь, что вы в папке проекта")
		os.Exit(1)
	}
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

// Выполнено с любовью для Босса 🐈‍