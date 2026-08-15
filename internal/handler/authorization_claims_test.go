package handler_test

import (
	"testing"

	"github.com/sekolahpintar/ews-worker/internal/model"
)

// ─── CanReadSiswa unit test ───────────────────────────────────────────────

func TestEWSCanReadSiswa(t *testing.T) {
	s10 := int64(10)
	tests := []struct {
		name    string
		claims  *model.UserClaims
		siswaID int64
		want    bool
	}{
		{"nil claims", nil, 10, false},
		{"admin any siswa", &model.UserClaims{Roles: []string{"admin"}}, 99, true},
		{"staff any siswa", &model.UserClaims{Roles: []string{"staff"}}, 99, true},
		{"guru any siswa", &model.UserClaims{Roles: []string{"guru"}}, 99, true},
		{"siswa own", &model.UserClaims{Roles: []string{"siswa"}, SiswaID: &s10}, 10, true},
		{"siswa other", &model.UserClaims{Roles: []string{"siswa"}, SiswaID: &s10}, 99, false},
		{"wali ward", &model.UserClaims{Roles: []string{"wali"}, WaliSiswaIDs: []int64{10, 20}}, 20, true},
		{"wali non-ward", &model.UserClaims{Roles: []string{"wali"}, WaliSiswaIDs: []int64{10, 20}}, 99, false},
		{"unknown role", &model.UserClaims{Roles: []string{"external"}}, 10, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.claims.CanReadSiswa(tc.siswaID)
			if got != tc.want {
				t.Errorf("CanReadSiswa(%d) = %v, want %v", tc.siswaID, got, tc.want)
			}
		})
	}
}

// ─── Role methods unit test ───────────────────────────────────────────────

func TestEWSRoleMethods(t *testing.T) {
	tests := []struct {
		roles   []string
		isAdmin bool
		isGuru  bool
		isSiswa bool
		isWali  bool
	}{
		{[]string{"admin"}, true, false, false, false},
		{[]string{"staff"}, true, false, false, false},
		{[]string{"guru"}, false, true, false, false},
		{[]string{"siswa"}, false, false, true, false},
		{[]string{"wali"}, false, false, false, true},
	}
	for _, tc := range tests {
		c := &model.UserClaims{Roles: tc.roles}
		if c.IsAdmin() != tc.isAdmin {
			t.Errorf("roles=%v IsAdmin got %v want %v", tc.roles, c.IsAdmin(), tc.isAdmin)
		}
		if c.IsGuru() != tc.isGuru {
			t.Errorf("roles=%v IsGuru got %v want %v", tc.roles, c.IsGuru(), tc.isGuru)
		}
		if c.IsSiswa() != tc.isSiswa {
			t.Errorf("roles=%v IsSiswa got %v want %v", tc.roles, c.IsSiswa(), tc.isSiswa)
		}
		if c.IsWali() != tc.isWali {
			t.Errorf("roles=%v IsWali got %v want %v", tc.roles, c.IsWali(), tc.isWali)
		}
	}
}
