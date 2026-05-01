package auth

import (
	"aidanwoods.dev/go-paseto"
)

// GenerateSecretKey создаёт новый случайный ключ и возвращает его в hex
func GenerateSecretKey() (string, error) {
    key := paseto.NewV4SymmetricKey()
    return key.ExportHex(), nil
}

// Выполнено с любовью для Босса 🐈‍