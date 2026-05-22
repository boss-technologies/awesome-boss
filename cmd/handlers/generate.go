package handlers

import (
	"flag"
	"fmt"
	"os"

	"github.com/boss-technologies/awesome-boss/auth"
)

// ---------- 2. КОМАНДА generate ----------
func HandleGenerate(args []string) {
	if len(args) == 0 {
		fmt.Println("❌ Укажите подкоманду: boss generate key")
		os.Exit(1)
	}
	switch args[0] {
	case "key":
		handleGenerateKey(args[1:])
	default:
		fmt.Printf("❌ Неизвестная подкоманда: %s\n", args[0])
		os.Exit(1)
	}
}

func handleGenerateKey(args []string) {
	fs := flag.NewFlagSet("generate key", flag.ExitOnError)
	jsonFlag := fs.Bool("json", false, "вывод в JSON формате")
	fs.Parse(args)

	hexKey, err := auth.GenerateSecretKey()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка генерации ключа: %v\n", err)
		os.Exit(1)
	}

	if *jsonFlag {
		fmt.Printf(`{"key": "%s", "note": "Сохраните этот ключ в переменную BOSS_AUTH_SECRET"}`+"\n", hexKey)
	} else {
		fmt.Println(hexKey)
		fmt.Println("# Сохраните этот ключ в переменную окружения BOSS_AUTH_SECRET или в config.toml")
	}
}

// Выполнено с любовью для Босса 🐈‍