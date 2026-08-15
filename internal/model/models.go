package model

import (
	"strings"
	"time"
)

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
	ID         int64      `db:"id"           json:"id"`
	MstSiswaID int64      `db:"mst_siswa_id" json:"mst_siswa_id"`
	Kategori   string     `db:"kategori"     json:"kategori"`
	Level      int32      `db:"level"        json:"level"`
	Pesan      string     `db:"pesan"        json:"pesan"`
	IsResolved bool       `db:"is_resolved"  json:"is_resolved"`
	ResolvedAt *time.Time `db:"resolved_at"  json:"resolved_at"`
	ResolvedBy *int64     `db:"resolved_by"  json:"resolved_by,omitempty"`
	CreatedAt  *time.Time `db:"created_at"   json:"created_at"`
	UpdatedAt  *time.Time `db:"updated_at"   json:"updated_at"`
}

// ---- JWT Claims ----

type UserClaims struct {
	UserID       int64
	Roles        []string
	SiswaID      *int64
	GuruID       *int64
	WaliID       *int64
	WaliSiswaIDs []int64
	DbSchema     string // tenant PostgreSQL schema
}

func (c *UserClaims) HasRole(role string) bool {
	target := strings.ToLower(role)
	for _, r := range c.Roles {
		if strings.ToLower(r) == target {
			return true
		}
	}
	return false
}

func (c *UserClaims) IsAdmin() bool {
	return c.HasRole("admin") || c.HasRole("superadmin") || c.HasRole("staff")
}

func (c *UserClaims) IsGuru() bool {
	return c.HasRole("guru")
}

func (c *UserClaims) IsSiswa() bool {
	return c.HasRole("siswa")
}

func (c *UserClaims) IsWali() bool {
	return c.HasRole("wali")
}

// CanReadSiswa checks if the caller is allowed to read EWS data for the given siswa.
// Admin/guru: true. Siswa: only own ID. Wali: only ward IDs.
func (c *UserClaims) CanReadSiswa(siswaID int64) bool {
	if c == nil {
		return false
	}
	if c.IsAdmin() || c.IsGuru() {
		return true
	}
	if c.IsSiswa() && c.SiswaID != nil && *c.SiswaID == siswaID {
		return true
	}
	if c.IsWali() {
		for _, id := range c.WaliSiswaIDs {
			if id == siswaID {
				return true
			}
		}
	}
	return false
}
