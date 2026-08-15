package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/sekolahpintar/ews-worker/internal/model"
)

type EWSRepo struct {
	db *sqlx.DB
}

func NewEWSRepo(db *sqlx.DB) *EWSRepo {
	return &EWSRepo{db: db}
}

// CountAbsensiAlpha counts alpha absences (status=4) in the last N days for a student.
func (r *EWSRepo) CountAbsensiAlpha(ctx context.Context, siswaID int64, days int) (int, error) {
	var count int
	query := fmt.Sprintf(`
		SELECT COUNT(*) FROM trx_absensi_siswa
		WHERE mst_siswa_id = $1
		  AND status = 4
		  AND tanggal >= CURRENT_DATE - INTERVAL '%d days'
		  AND deleted_at IS NULL
	`, days)
	err := r.db.QueryRowContext(ctx, query, siswaID).Scan(&count)
	return count, err
}

// AvgNilai returns the average nilai for a student in the last N days. Returns -1 if no records.
func (r *EWSRepo) AvgNilai(ctx context.Context, siswaID int64, days int) (float64, error) {
	var avg *float64
	query := fmt.Sprintf(`
		SELECT AVG(nilai) FROM trx_nilai
		WHERE mst_siswa_id = $1
		  AND created_at >= NOW() - INTERVAL '%d days'
		  AND deleted_at IS NULL
	`, days)
	err := r.db.QueryRowContext(ctx, query, siswaID).Scan(&avg)
	if err != nil {
		return -1, err
	}
	if avg == nil {
		return -1, nil
	}
	return *avg, nil
}

// CountBKKasus counts BK cases in the last N days for a student.
func (r *EWSRepo) CountBKKasus(ctx context.Context, siswaID int64, days int) (int, error) {
	var count int
	query := fmt.Sprintf(`
		SELECT COUNT(*) FROM trx_bk_kasus
		WHERE mst_siswa_id = $1
		  AND created_at >= NOW() - INTERVAL '%d days'
		  AND deleted_at IS NULL
	`, days)
	err := r.db.QueryRowContext(ctx, query, siswaID).Scan(&count)
	return count, err
}

// UpsertAlert creates or updates an open EWS alert for a student+kategori.
func (r *EWSRepo) UpsertAlert(ctx context.Context, siswaID int64, kategori string, level int32, pesan string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO trx_ews_alerts (mst_siswa_id, kategori, level, pesan, is_resolved, created_at, updated_at)
		VALUES ($1, $2, $3, $4, false, NOW(), NOW())
		ON CONFLICT (mst_siswa_id, kategori)
		  WHERE deleted_at IS NULL AND is_resolved = false
		DO UPDATE
		  SET level      = EXCLUDED.level,
		      pesan      = EXCLUDED.pesan,
		      updated_at = NOW()
	`, siswaID, kategori, level, pesan)
	if err != nil {
		// Fallback: partial index may not exist — plain insert ignoring conflict
		_, err = r.db.ExecContext(ctx, `
			INSERT INTO trx_ews_alerts (mst_siswa_id, kategori, level, pesan, is_resolved, created_at, updated_at)
			VALUES ($1, $2, $3, $4, false, NOW(), NOW())
			ON CONFLICT DO NOTHING
		`, siswaID, kategori, level, pesan)
	}
	return err
}

// AutoResolve resolves an open EWS alert by student ID and kategori.
func (r *EWSRepo) AutoResolve(ctx context.Context, siswaID int64, kategori string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE trx_ews_alerts
		SET is_resolved = true, resolved_at = NOW(), updated_at = NOW()
		WHERE mst_siswa_id = $1 AND kategori = $2 AND is_resolved = false AND deleted_at IS NULL
	`, siswaID, kategori)
	return err
}

// FindByID fetches a single alert by its primary key.
func (r *EWSRepo) FindByID(ctx context.Context, id int64) (*model.TrxEWSAlert, error) {
	var a model.TrxEWSAlert
	err := r.db.QueryRowxContext(ctx, `
		SELECT id, mst_siswa_id, kategori, level, pesan, is_resolved, resolved_at, resolved_by, created_at, updated_at
		FROM trx_ews_alerts
		WHERE id = $1 AND deleted_at IS NULL
	`, id).StructScan(&a)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return &a, err
}

// ResolveByID resolves a specific EWS alert by primary key and records the resolving actor.
func (r *EWSRepo) ResolveByID(ctx context.Context, id int64, resolvedByUserID int64) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE trx_ews_alerts
		SET is_resolved = true,
		    resolved_at = NOW(),
		    resolved_by = $2,
		    updated_at  = NOW()
		WHERE id = $1 AND is_resolved = false AND deleted_at IS NULL
	`, id, resolvedByUserID)
	if err != nil {
		// Fallback for schema without resolved_by column
		res, err = r.db.ExecContext(ctx, `
			UPDATE trx_ews_alerts
			SET is_resolved = true, resolved_at = NOW(), updated_at = NOW()
			WHERE id = $1 AND is_resolved = false AND deleted_at IS NULL
		`, id)
		if err != nil {
			return err
		}
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListAlerts returns paginated EWS alerts with optional filters.
func (r *EWSRepo) ListAlerts(ctx context.Context, filters map[string]string, page, perPage int) ([]model.TrxEWSAlert, int, error) {
	where := "WHERE deleted_at IS NULL"
	args := []interface{}{}

	if v := filters["mst_siswa_id"]; v != "" {
		args = append(args, v)
		where += fmt.Sprintf(" AND mst_siswa_id = $%d", len(args))
	}
	if v := filters["is_resolved"]; v != "" {
		args = append(args, v == "true")
		where += fmt.Sprintf(" AND is_resolved = $%d", len(args))
	}
	if v := filters["level"]; v != "" {
		args = append(args, v)
		where += fmt.Sprintf(" AND level = $%d", len(args))
	}
	if v := filters["kategori"]; v != "" {
		args = append(args, v)
		where += fmt.Sprintf(" AND kategori = $%d", len(args))
	}

	var total int
	if err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM trx_ews_alerts "+where,
		args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * perPage
	args = append(args, perPage, offset)

	rows, err := r.db.QueryxContext(ctx,
		"SELECT id, mst_siswa_id, kategori, level, pesan, is_resolved, resolved_at, created_at, updated_at "+
			"FROM trx_ews_alerts "+where+
			fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args)),
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []model.TrxEWSAlert
	for rows.Next() {
		var a model.TrxEWSAlert
		if err := rows.StructScan(&a); err != nil {
			return nil, 0, err
		}
		list = append(list, a)
	}
	return list, total, nil
}

// GetAlertsBySiswa returns all EWS alerts for a specific student.
func (r *EWSRepo) GetAlertsBySiswa(ctx context.Context, siswaID int64) ([]model.TrxEWSAlert, error) {
	rows, err := r.db.QueryxContext(ctx, `
		SELECT id, mst_siswa_id, kategori, level, pesan, is_resolved, resolved_at, created_at, updated_at
		FROM trx_ews_alerts
		WHERE mst_siswa_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`, siswaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.TrxEWSAlert
	for rows.Next() {
		var a model.TrxEWSAlert
		if err := rows.StructScan(&a); err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	return list, nil
}
