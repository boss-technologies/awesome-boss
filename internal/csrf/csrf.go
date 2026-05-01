package csrf

import (
    "crypto/hmac"
    "crypto/rand"
    "crypto/sha256"
    "encoding/hex"
    "strings"

    "github.com/boss-technologies/awesome-boss/core"
    "github.com/valyala/fasthttp"
)

// CSRFMiddleware проверяет токен при небезопасных методах.
// secretKey — 32-байтовый ключ для HMAC.
func CSRFMiddleware(secretKey []byte) core.Middleware {
    if len(secretKey) != 32 {
        panic("csrf: secretKey must be 32 bytes")
    }

    return func(next core.Handler) core.Handler {
        return func(ctx *core.BossContext) error {
            if isSafeMethod(ctx.Method()) {
                return next(ctx)
            }

            // 1. Читаем подписанный токен из куки
            signedCookieToken := string(ctx.Request.Header.Cookie("csrf_token"))
            if signedCookieToken == "" {
                return forbidden(ctx, "missing CSRF cookie")
            }

            // 2. Читаем токен из кастомного заголовка
            headerToken := string(ctx.Request.Header.Peek("X-CSRF-Token"))
            if headerToken == "" {
                return forbidden(ctx, "missing X-CSRF-Token header")
            }

            // 3. Сравниваем их (защита от timing-атак)
            if !hmac.Equal([]byte(signedCookieToken), []byte(headerToken)) {
                return forbidden(ctx, "CSRF token mismatch")
            }

            // 4. Проверяем подпись
            if !verifyHMACToken(signedCookieToken, secretKey) {
                return forbidden(ctx, "invalid token signature")
            }

            return next(ctx)
        }
    }
}

// SetCSRFTokenMiddleware генерирует и устанавливает CSRF-токен в куку,
// если её ещё нет, и сохраняет сырой токен в контексте для шаблонов.
func SetCSRFTokenMiddleware(secretKey []byte) core.Middleware {
    if len(secretKey) != 32 {
        panic("csrf: secretKey must be 32 bytes")
    }

    return func(next core.Handler) core.Handler {
        return func(ctx *core.BossContext) error {
            // Работаем только на чтение (GET/HEAD), чтобы установить токен
            if !isSafeMethod(ctx.Method()) {
                return next(ctx)
            }

            // Если кука уже есть — не перезаписываем
            if len(ctx.Request.Header.Cookie("csrf_token")) > 0 {
                return next(ctx)
            }

            // Генерируем новый подписанный токен
            signedToken := GenerateToken(secretKey)

            // Устанавливаем защищённую куку
            cookie := fasthttp.Cookie{}
            cookie.SetKey("csrf_token")
            cookie.SetValue(signedToken)
            cookie.SetPath("/")
            cookie.SetSecure(true)        // только по HTTPS
            cookie.SetSameSite(fasthttp.CookieSameSiteStrictMode)
            cookie.SetHTTPOnly(false)      // важно: чтобы JS мог читать токен
            ctx.Response.Header.SetCookie(&cookie)

            // Сохраняем сырой токен (до подписи) в контексте,
            // чтобы шаблоны Fur могли вставить его в форму / meta-тег
            parts := strings.SplitN(signedToken, ".", 2)
            if len(parts) == 2 {
                ctx.Set("csrf_token_raw", parts[0])
            }

            return next(ctx)
        }
    }
}

// GenerateToken создаёт подписанный токен в формате "rawToken.hmacSignature".
func GenerateToken(secretKey []byte) string {
    b := make([]byte, 32)
    rand.Read(b)
    rawToken := hex.EncodeToString(b)
    signature := computeHMAC(rawToken, secretKey)
    return rawToken + "." + signature
}

func computeHMAC(message string, key []byte) string {
    mac := hmac.New(sha256.New, key)
    mac.Write([]byte(message))
    return hex.EncodeToString(mac.Sum(nil))
}

func verifyHMACToken(signedToken string, key []byte) bool {
    parts := strings.SplitN(signedToken, ".", 2)
    if len(parts) != 2 {
        return false
    }
    expected := computeHMAC(parts[0], key)
    return hmac.Equal([]byte(parts[1]), []byte(expected))
}

func isSafeMethod(method string) bool {
    m := string(method)
    return m == "GET" || m == "HEAD"
}

func forbidden(ctx *core.BossContext, msg string) error {
    ctx.Response.SetStatusCode(fasthttp.StatusForbidden)
    ctx.Response.SetBodyString("Forbidden - " + msg)
    return nil
}