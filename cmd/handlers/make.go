package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/boss-technologies/awesome-boss/internal/bossq"
)

// ---------- 4. КОМАНДА make (генерация Store) ----------
func HandleMakeModels() {
	root, _ := os.Getwd()
	fmt.Printf("🔍 BossQ сканирует все папки в '%s'...\n", root)
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return nil
		}
		if strings.HasPrefix(info.Name(), ".") || info.Name() == "vendor" {
			return filepath.SkipDir
		}
		if !hasGoFiles(path) {
			return nil
		}
		if err := bossq.Generate(path); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️ Ошибка в %s: %v\n", path, err)
		}
		return nil
	})
	fmt.Println("✅ BossQ завершил сканирование!")
}

func hasGoFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
			return true
		}
	}
	return false
}

// Выполнено с любовью для Босса 🐈‍
