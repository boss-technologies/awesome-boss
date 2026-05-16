// config/config_test.go
package config

import (
    "os"
    "path/filepath"
    "testing"
    "time"
)

func TestLoadConfig_ValidFile(t *testing.T) {
    tmpDir := t.TempDir()
    configPath := filepath.Join(tmpDir, "boss.toml")
    content := `
        mode = "fintech"

        [server]
        port = 3000
        read_timeout = "10s"
        write_timeout = "20s"
        enable_gzip = false

        [auth]
        secret_key = "fromfile"

        [database]
        url_database = "postgres://user:pass@localhost:5432/test?sslmode=disable"

        [logging]
        level = "debug"
    `
    err := os.WriteFile(configPath, []byte(content), 0644)
    if err != nil {
        t.Fatal(err)
    }

    cfg, err := LoadConfig(configPath)
    if err != nil {
        t.Fatalf("LoadConfig вернул ошибку: %v", err)
    }

    // Проверяем значения из файла
    if cfg.Mode != ModeFintech {
        t.Errorf("Mode = %v, ожидается fintech", cfg.Mode)
    }
    if cfg.Server.Port != 3000 {
        t.Errorf("Port = %d, ожидается 3000", cfg.Server.Port)
    }
    if cfg.Server.ReadTimeout != 10*time.Second {
        t.Errorf("ReadTimeout = %v, ожидается 10s", cfg.Server.ReadTimeout)
    }
    if cfg.Server.EnableGzip != false {
        t.Error("EnableGzip должен быть false")
    }
    if cfg.Auth.SecretKey != "fromfile" {
        t.Errorf("SecretKey = %s, ожидается fromfile", cfg.Auth.SecretKey)
    }
    if cfg.Database.Url != "postgres://user:pass@localhost:5432/test?sslmode=disable" {
        t.Errorf("Database.Url не совпадает")
    }
}

func TestLoadConfig_EnvOverride(t *testing.T) {
    t.Setenv("BOSS_AUTH_SECRET", "secretfromenv")
    t.Setenv("DATABASE_URL", "postgres://env:pass@localhost/envdb")

    tmpDir := t.TempDir()
    configPath := filepath.Join(tmpDir, "boss.toml")
    content := `
        [auth]
        secret_key = "fromfile"
        [database]
        url_database = "postgres://file:pass@localhost/filedb"
    `
    os.WriteFile(configPath, []byte(content), 0644)

    cfg, err := LoadConfig(configPath)
    if err != nil {
        t.Fatal(err)
    }

    if cfg.Auth.SecretKey != "secretfromenv" {
        t.Errorf("SecretKey = %s, ожидается secretfromenv", cfg.Auth.SecretKey)
    }
    if cfg.Database.Url != "postgres://env:pass@localhost/envdb" {
        t.Errorf("Database.Url = %s, ожидается из env", cfg.Database.Url)
    }
}

func TestLoadConfig_MissingFile(t *testing.T) {
    _, err := LoadConfig("/nonexistent/path.toml")
    if err == nil {
        t.Error("Должна быть ошибка при отсутствии файла")
    }
}

func TestValidate_FintechForbidsInsecureSSL(t *testing.T) {
    cfg := Default()
    cfg.Mode = ModeFintech
    cfg.Database.Url = "postgres://user:pass@localhost/db?sslmode=disable"
    err := cfg.Validate()
    if err == nil {
        t.Error("Validate должна вернуть ошибку для sslmode=disable в fintech режиме")
    }
}

func TestValidate_FintechAllowsSSLVerifyFull(t *testing.T) {
    cfg := Default()
    cfg.Mode = ModeFintech
    cfg.Database.Url = "postgres://user:pass@localhost/db?sslmode=verify-full"
    err := cfg.Validate()
    if err != nil {
        t.Errorf("Ожидался nil, получили %v", err)
    }
}