package handlers

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// ---------- 1. КОМАНДА build ----------
func HandleBuild(args []string) {
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

func getExeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// Выполнено с любовью для Босса 🐈‍