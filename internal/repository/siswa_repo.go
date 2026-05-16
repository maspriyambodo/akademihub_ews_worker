package repository

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/sekolahpintar/ews-worker/internal/model"
)

type SiswaRepo struct {
	db *sqlx.DB
}

func NewSiswaRepo(db *sqlx.DB) *SiswaRepo {
	return &SiswaRepo{db: db}
}

// ListAktif returns all active students (status = 1).
func (r *SiswaRepo) ListAktif(ctx context.Context) ([]model.MstSiswa, error) {
	rows, err := r.db.QueryxContext(ctx, `
		SELECT id, nis, nama, status, mst_kelas_id
		FROM mst_siswa
		WHERE status = 1 AND deleted_at IS NULL
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.MstSiswa
	for rows.Next() {
		var s model.MstSiswa
		if err := rows.StructScan(&s); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, nil
}

// FindByID returns a single student by ID.
func (r *SiswaRepo) FindByID(ctx context.Context, id int64) (*model.MstSiswa, error) {
	var s model.MstSiswa
	err := r.db.QueryRowxContext(ctx, r.db.Rebind(`
		SELECT id, nis, nama, status, mst_kelas_id
		FROM mst_siswa
		WHERE id = ? AND deleted_at IS NULL
	`), id).StructScan(&s)
	if err != nil {
		return nil, ErrNotFound
	}
	return &s, nil
}
