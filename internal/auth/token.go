package auth

import (
    "encoding/hex"
    "fmt"
    "time"

    "aidanwoods.dev/go-paseto"
)

// decodeKey преобразует hex-строку в 32-байтовый ключ
func decodeKey(hexKey string) ([]byte, error) {
    key, err := hex.DecodeString(hexKey)
    if err != nil {
        return nil, fmt.Errorf("invalid hex key: %w", err)
    }
    if len(key) != 32 {
        return nil, fmt.Errorf("key must be 32 bytes, got %d", len(key))
    }
    return key, nil
}

// GenerateToken создаёт PASETO-токен с 24-часовым сроком жизни.
// secretKeyHex – 32-байтовый ключ в hex-формате.
func GenerateToken[T ~uint64 | ~string](secretKeyHex string, userID T) (string, error)  {
    keyBytes, err := decodeKey(secretKeyHex)
    if err != nil {
        return "", err
    }

    key, err := paseto.V4SymmetricKeyFromBytes(keyBytes)
    if err != nil {
        return "", fmt.Errorf("failed to import key: %w", err)
    }

    token := paseto.NewToken()
    token.SetIssuedAt(time.Now())
    token.SetNotBefore(time.Now())
    token.SetExpiration(time.Now().Add(24 * time.Hour))
    token.SetString("user_id", fmt.Sprintf("%v", userID))

    return token.V4Encrypt(key, nil), nil
}

// ValidateToken проверяет токен и возвращает user_id (строку).
func ValidateToken(secretKeyHex string, tokenString string) (string, error) {
    keyBytes, err := decodeKey(secretKeyHex)
    if err != nil {
        return "", err
    }

    key, err := paseto.V4SymmetricKeyFromBytes(keyBytes)
    if err != nil {
        return "", fmt.Errorf("failed to import key: %w", err)
    }

    parser := paseto.NewParser()
    parsedToken, err := parser.ParseV4Local(key, tokenString, nil)
    if err != nil {
        return "", err
    }

    userID, err := parsedToken.GetString("user_id")
    if err != nil {
        return "", fmt.Errorf("token missing user_id: %w", err)
    }
    return userID, nil
}

// Выполнено с любовью для Босса 🐈‍