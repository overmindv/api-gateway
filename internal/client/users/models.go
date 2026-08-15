package users

import "context"

type UserService interface {
	Register(context.Context, RegisterInput) (*AuthPayload, error)
	Login(context.Context, LoginInput) (*AuthPayload, error)
	GetUser(context.Context, string) (*User, error)
	GetUserByUsername(context.Context, string) (*User, error)
	ListUsers(context.Context, string, int, int) ([]*User, error)
	UpdateUser(context.Context, string, UpdateUserInput) (*User, error)
	DeleteUser(context.Context, string) (bool, error)
	SetUserAdmin(context.Context, string, bool) (*User, error)
	SetUserAdminByUsername(context.Context, string, bool) (*User, error)
}

type User struct {
	ID          string   `json:"id"`
	Email       string   `json:"email"`
	Username    string   `json:"username"`
	FirstName   string   `json:"firstName"`
	LastName    string   `json:"lastName"`
	BirthDate   *string  `json:"birthDate"`
	Phone       *string  `json:"phone"`
	Roles       []string `json:"roles"`
	IsAdmin     bool     `json:"isAdmin"`
	IsSuperuser bool     `json:"isSuperuser"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
}

type AuthPayload struct {
	User      *User  `json:"user"`
	Token     string `json:"token"`
	ExpiresAt string `json:"expiresAt"`
}

type RegisterInput struct {
	Email     string  `json:"email"`
	Password  string  `json:"password"`
	Username  string  `json:"username"`
	FirstName string  `json:"firstName"`
	LastName  string  `json:"lastName"`
	BirthDate *string `json:"birthDate,omitempty"`
	Phone     string  `json:"phone"`
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UpdateUserInput struct {
	Username       *string `json:"username,omitempty"`
	FirstName      *string `json:"firstName,omitempty"`
	LastName       *string `json:"lastName,omitempty"`
	BirthDate      *string `json:"birthDate,omitempty"`
	ClearBirthDate bool    `json:"clearBirthDate"`
	Phone          *string `json:"phone,omitempty"`
}

type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }
