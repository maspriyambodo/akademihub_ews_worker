package middleware

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jmoiron/sqlx"
	"github.com/sekolahpintar/ews-worker/internal/model"
)

func jsonErr(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = fmt.Fprintf(w, `{"success":false,"message":%q}`, msg)
}

type contextKey string

const claimsKey contextKey = "user_claims"

// ClaimsContextKey returns the context key used to store UserClaims.
// Exported so tests can inject fake claims without running the full auth middleware.
func ClaimsContextKey() interface{} {
	return claimsKey
}

// Auth validates the Bearer JWT token and loads user claims into context.
func Auth(jwtSecret string, db *sqlx.DB) func(http.Handler) http.Handler {
	secretBytes := []byte(jwtSecret)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				jsonErr(w, "Unauthenticated", http.StatusUnauthorized)
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
				}
				return secretBytes, nil
			}, jwt.WithExpirationRequired())

			if err != nil || !token.Valid {
				jsonErr(w, "Token invalid or expired", http.StatusUnauthorized)
				return
			}

			mapClaims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				jsonErr(w, "Invalid token claims", http.StatusUnauthorized)
				return
			}

			// sub can be a string (Lcobucci) or float64 (generic JSON number)
			var userID int64
			switch v := mapClaims["sub"].(type) {
			case string:
				fmt.Sscanf(v, "%d", &userID)
			case float64:
				userID = int64(v)
			}
			if userID == 0 {
				jsonErr(w, "Invalid token sub", http.StatusUnauthorized)
				return
			}

			claims, err := loadUserClaims(r.Context(), db, userID)
			if err != nil {
				jsonErr(w, "Unauthenticated", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetClaims retrieves UserClaims from request context.
func GetClaims(ctx context.Context) *model.UserClaims {
	c, _ := ctx.Value(claimsKey).(*model.UserClaims)
	return c
}

func loadUserClaims(ctx context.Context, db *sqlx.DB, userID int64) (*model.UserClaims, error) {
	var active int8
	err := db.QueryRowContext(ctx,
		db.Rebind(`SELECT is_active FROM sys_users WHERE id = ? AND deleted_at IS NULL`),
		userID,
	).Scan(&active)
	if err == sql.ErrNoRows || active == 0 {
		return nil, fmt.Errorf("user not found or inactive")
	}
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, db.Rebind(`
		SELECT r.name
		FROM sys_user_roles ur
		JOIN sys_roles r ON r.id = ur.sys_role_id
		WHERE ur.sys_user_id = ? AND ur.deleted_at IS NULL
	`), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []string
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}

	claims := &model.UserClaims{UserID: userID, Roles: roles}

	// Load tenant schema for multi-tenant DB routing
	var dbSchema string
	_ = db.QueryRowContext(ctx, db.Rebind(`
		SELECT s.db_schema
		FROM mst_sekolah s
		JOIN sys_users u ON u.mst_sekolah_id = s.id
		WHERE u.id = ? AND u.deleted_at IS NULL AND s.deleted_at IS NULL
		LIMIT 1
	`), userID).Scan(&dbSchema)
	claims.DbSchema = dbSchema

	// Load profile IDs
	var siswaID int64
	if err := db.QueryRowContext(ctx, db.Rebind(`SELECT id FROM mst_siswa WHERE sys_user_id = ? AND deleted_at IS NULL LIMIT 1`), userID).Scan(&siswaID); err == nil {
		claims.SiswaID = &siswaID
	}

	var guruID int64
	if err := db.QueryRowContext(ctx, db.Rebind(`SELECT id FROM mst_guru WHERE sys_user_id = ? AND deleted_at IS NULL LIMIT 1`), userID).Scan(&guruID); err == nil {
		claims.GuruID = &guruID
	}

	var waliID int64
	if err := db.QueryRowContext(ctx, db.Rebind(`SELECT id FROM mst_wali_siswa WHERE sys_user_id = ? AND deleted_at IS NULL LIMIT 1`), userID).Scan(&waliID); err == nil {
		claims.WaliID = &waliID
		rows, wErr := db.QueryContext(ctx, db.Rebind(`
			SELECT mst_siswa_id FROM trx_relasi_wali_siswa
			WHERE mst_wali_siswa_id = ? AND deleted_at IS NULL
		`), waliID)
		if wErr == nil {
			defer rows.Close()
			for rows.Next() {
				var sid int64
				if scanErr := rows.Scan(&sid); scanErr == nil {
					claims.WaliSiswaIDs = append(claims.WaliSiswaIDs, sid)
				}
			}
		}
	}

	return claims, nil
}
