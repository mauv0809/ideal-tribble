package web

import (
	"net/http"
	"strconv"

	"github.com/mauv0809/ideal-tribble/internal/auth"
	"github.com/mauv0809/ideal-tribble/internal/web/templates"
)

// AdminHandlers handles admin-related HTTP requests.
type AdminHandlers struct {
	middleware *Middleware
	authStore  *auth.Store
}

// NewAdminHandlers creates a new admin handlers instance.
func NewAdminHandlers(middleware *Middleware, authStore *auth.Store) *AdminHandlers {
	return &AdminHandlers{
		middleware: middleware,
		authStore:  authStore,
	}
}

// UserListPage renders the user management page.
func (h *AdminHandlers) UserListPage(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)

	users, err := h.authStore.ListUsers()
	if err != nil {
		http.Error(w, "Failed to load users", http.StatusInternalServerError)
		return
	}

	data := templates.UserListData{
		PageData: templates.PageData{
			Title: "User Management",
			User:  user,
		},
		Users: users,
	}

	if flashes := h.middleware.GetFlash(w, r, "success"); len(flashes) > 0 {
		data.PageData.FlashSuccess = flashes[0]
	}
	if flashes := h.middleware.GetFlash(w, r, "error"); len(flashes) > 0 {
		data.PageData.FlashError = flashes[0]
	}

	component := templates.UserListPage(data)
	_ = component.Render(r.Context(), w)
}

// UsersTablePartial renders just the users table for htmx updates.
func (h *AdminHandlers) UsersTablePartial(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)
	users, _ := h.authStore.ListUsers()
	component := templates.UsersTable(users, user.ID)
	_ = component.Render(r.Context(), w)
}

// NewUserPage renders the add new user form.
func (h *AdminHandlers) NewUserPage(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)

	data := templates.NewUserData{
		PageData: templates.PageData{
			Title: "Add User",
			User:  user,
		},
	}

	component := templates.NewUserPage(data)
	_ = component.Render(r.Context(), w)
}

// CreateUser handles adding a new user.
func (h *AdminHandlers) CreateUser(w http.ResponseWriter, r *http.Request) {
	currentUser := GetUser(r)

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")
	isAdmin := r.FormValue("is_admin") == "1"

	if email == "" || password == "" {
		data := templates.NewUserData{
			PageData: templates.PageData{
				Title: "Add User",
				User:  currentUser,
			},
			Error: "Email and password are required",
		}
		component := templates.NewUserPage(data)
		_ = component.Render(r.Context(), w)
		return
	}

	if len(password) < 8 {
		data := templates.NewUserData{
			PageData: templates.PageData{
				Title: "Add User",
				User:  currentUser,
			},
			Error: "Password must be at least 8 characters",
		}
		component := templates.NewUserPage(data)
		_ = component.Render(r.Context(), w)
		return
	}

	_, err := h.authStore.CreateUser(email, password, isAdmin)
	if err != nil {
		errorMsg := "Failed to create user"
		if err == auth.ErrEmailAlreadyExists {
			errorMsg = "A user with this email already exists"
		}
		data := templates.NewUserData{
			PageData: templates.PageData{
				Title: "Add User",
				User:  currentUser,
			},
			Error: errorMsg,
		}
		component := templates.NewUserPage(data)
		_ = component.Render(r.Context(), w)
		return
	}

	_ = h.middleware.SetFlash(w, r, "success", "User created successfully")
	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

// DeleteUser handles removing a user.
func (h *AdminHandlers) DeleteUser(w http.ResponseWriter, r *http.Request) {
	currentUser := GetUser(r)

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	// Prevent deleting yourself
	if id == currentUser.ID {
		http.Error(w, "Cannot delete your own account", http.StatusBadRequest)
		return
	}

	_ = h.authStore.DeleteUser(id)

	// Return updated table for htmx
	h.UsersTablePartial(w, r)
}
