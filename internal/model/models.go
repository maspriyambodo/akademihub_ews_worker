package model

import "time"

// ---- mst_siswa ----

type MstSiswa struct {
	ID      int64  `db:"id"`
	NIS     string `db:"nis"`
	Nama    string `db:"nama"`
	Status  int16  `db:"status"`
	KelasID *int64 `db:"mst_kelas_id"`
}

// ---- trx_ews_alerts ----

type TrxEWSAlert struct {
	ID         int64      `db:"id"          json:"id"`
	MstSiswaID int64      `db:"mst_siswa_id" json:"mst_siswa_id"`
	Kategori   string     `db:"kategori"    json:"kategori"`
	Level      int32      `db:"level"       json:"level"`
	Pesan      string     `db:"pesan"       json:"pesan"`
	IsResolved bool       `db:"is_resolved" json:"is_resolved"`
	ResolvedAt *time.Time `db:"resolved_at" json:"resolved_at"`
	CreatedAt  *time.Time `db:"created_at"  json:"created_at"`
	UpdatedAt  *time.Time `db:"updated_at"  json:"updated_at"`
}

// ---- JWT Claims ----

type UserClaims struct {
	UserID   int64
	Roles    []string
	SiswaID  *int64
	GuruID   *int64
	DbSchema string // tenant PostgreSQL schema
}

func (c *UserClaims) HasRole(role string) bool {
	for _, r := range c.Roles {
		if r == role {
			return true
		}
	}
	return false
}

func (c *UserClaims) IsAdmin() bool {
	return c.HasRole("admin") || c.HasRole("superadmin")
}
