// Package userdata implements the user data subsystem (email/phone storage),
// mirroring the original TS src/user-data.ts. Data is stored in the User table
// via GORM.
package userdata

import (
	"regexp"
	"strings"

	"gorm.io/gorm"

	"github.com/steemit/conveyor/internal/jsonrpc"
	"github.com/steemit/conveyor/internal/models"
)

var (
	emailPattern = regexp.MustCompile(`.+@.+\..+`)
	phonePattern = regexp.MustCompile(`^\+[0-9]+$`)
)

// UserData holds the dependencies for user-data operations.
type UserData struct {
	db        *gorm.DB
	adminRole string
}

// New creates a UserData instance.
func New(db *gorm.DB, adminRole string) *UserData {
	return &UserData{db: db, adminRole: adminRole}
}

// Register registers all user-data RPC methods.
func (u *UserData) Register(rpc *jsonrpc.Server) {
	rpc.RegisterAuthenticated("conveyor.get_user_data", u.getUserData)
	rpc.RegisterAuthenticated("conveyor.set_user_data", u.setUserData)
	rpc.RegisterAuthenticated("conveyor.is_email_registered", u.isEmailRegistered)
	rpc.RegisterAuthenticated("conveyor.is_phone_registered", u.isPhoneRegistered)
}

func (u *UserData) getUserData(ctx *jsonrpc.Context, req *jsonrpc.Request) (any, error) {
	var p struct {
		Account string `json:"account"`
	}
	if err := req.UnmarshalParams(&p); err != nil {
		return nil, err
	}
	if e := ctx.Assert(ctx.Account == p.Account || ctx.Account == u.adminRole, "Unauthorized"); e != nil {
		return nil, e
	}

	var user models.User
	result := u.db.WithContext(req.Ctx).First(&user, "account = ?", p.Account)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, jsonrpc.NewError(404, nil, "No such user")
		}
		return nil, jsonrpc.ErrInternalError(result.Error)
	}

	resp := map[string]any{}
	if user.Email != nil {
		resp["email"] = *user.Email
	} else {
		resp["email"] = nil
	}
	if user.Phone != nil {
		resp["phone"] = *user.Phone
	} else {
		resp["phone"] = nil
	}
	return resp, nil
}

func (u *UserData) setUserData(ctx *jsonrpc.Context, req *jsonrpc.Request) (any, error) {
	var p struct {
		Account  string         `json:"account"`
		UserData map[string]any `json:"userData"`
	}
	if err := req.UnmarshalParams(&p); err != nil {
		return nil, err
	}
	if e := ctx.Assert(ctx.Account == p.Account || ctx.Account == u.adminRole, "Unauthorized"); e != nil {
		return nil, e
	}
	if e := ctx.Assert(len(p.UserData) > 0, "no keys in data"); e != nil {
		return nil, e
	}

	// Extract and validate email/phone.
	var email, phone *string
	if v, ok := p.UserData["email"]; ok && v != nil {
		s := strings.TrimSpace(v.(string))
		if !emailPattern.MatchString(s) {
			return nil, jsonrpc.NewError(400, nil, "Invalid email format")
		}
		email = &s
	}
	if v, ok := p.UserData["phone"]; ok && v != nil {
		s := strings.TrimSpace(v.(string))
		if !phonePattern.MatchString(s) {
			return nil, jsonrpc.NewError(400, nil, "Invalid phone format: must be +[0-9]+")
		}
		phone = &s
	}

	// Upsert.
	var user models.User
	result := u.db.WithContext(req.Ctx).First(&user, "account = ?", p.Account)
	if result.Error == gorm.ErrRecordNotFound {
		// Create.
		user = models.User{Account: p.Account, Email: email, Phone: phone}
		if err := u.db.WithContext(req.Ctx).Create(&user).Error; err != nil {
			return nil, wrapDBError(err)
		}
	} else if result.Error != nil {
		return nil, jsonrpc.ErrInternalError(result.Error)
	} else {
		// Update.
		updates := map[string]any{}
		if email != nil {
			updates["email"] = *email
		}
		if phone != nil {
			updates["phone"] = *phone
		}
		if err := u.db.WithContext(req.Ctx).Model(&user).Updates(updates).Error; err != nil {
			return nil, wrapDBError(err)
		}
	}
	return map[string]any{}, nil
}

func (u *UserData) isEmailRegistered(ctx *jsonrpc.Context, req *jsonrpc.Request) (any, error) {
	var p struct {
		Email string `json:"email"`
	}
	if err := req.UnmarshalParams(&p); err != nil {
		return nil, err
	}
	if e := ctx.Assert(ctx.Account == u.adminRole, "Unauthorized"); e != nil {
		return nil, e
	}

	var count int64
	u.db.WithContext(req.Ctx).Model(&models.User{}).Where("email = ?", p.Email).Count(&count)
	return count > 0, nil
}

func (u *UserData) isPhoneRegistered(ctx *jsonrpc.Context, req *jsonrpc.Request) (any, error) {
	var p struct {
		Phone string `json:"phone"`
	}
	if err := req.UnmarshalParams(&p); err != nil {
		return nil, err
	}
	if e := ctx.Assert(ctx.Account == u.adminRole, "Unauthorized"); e != nil {
		return nil, e
	}

	var count int64
	u.db.WithContext(req.Ctx).Model(&models.User{}).Where("phone = ?", p.Phone).Count(&count)
	return count > 0, nil
}

// wrapDBError converts a GORM unique constraint error to a JsonRpcError 400
// with the validation error details, matching TS user-data.ts behavior.
func wrapDBError(err error) *jsonrpc.Error {
	// GORM wraps the underlying driver error. For SQLite/Postgres unique
	// violations, the error message contains "UNIQUE constraint" or
	// "duplicate key". We return a generic 400.
	s := err.Error()
	if strings.Contains(s, "UNIQUE") || strings.Contains(s, "duplicate key") || strings.Contains(s, "unique") {
		return jsonrpc.NewErrorWithData(400, err, "Validation error", map[string]any{
			"errors": []map[string]string{{"message": s}},
		})
	}
	return jsonrpc.ErrInternalError(err)
}
