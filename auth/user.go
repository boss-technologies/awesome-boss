package auth

type User struct {
    ID       uint64
    Username string
    PasswordHash string 
}

type UserStore interface {
    GetByID(id string) (*User, error)
}

// Выполнено с любовью для Босса 🐈‍