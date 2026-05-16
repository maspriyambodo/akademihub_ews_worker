package service

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/sekolahpintar/ews-worker/internal/model"
	"github.com/sekolahpintar/ews-worker/internal/repository"
)

const workerPoolSize = 50

// ProcessResult summarises the outcome of a batch EWS run.
type ProcessResult struct {
	Total     int `json:"total"`
	Processed int `json:"processed"`
	Errors    int `json:"errors"`
}

type EWSService struct {
	ewsRepo   *repository.EWSRepo
	siswaRepo *repository.SiswaRepo
}

func NewEWSService(ewsRepo *repository.EWSRepo, siswaRepo *repository.SiswaRepo) *EWSService {
	return &EWSService{ewsRepo: ewsRepo, siswaRepo: siswaRepo}
}

// ProcessAll runs the EWS check for every active student with a bounded worker pool.
func (s *EWSService) ProcessAll(ctx context.Context) (*ProcessResult, error) {
	siswaList, err := s.siswaRepo.ListAktif(ctx)
	if err != nil {
		return nil, fmt.Errorf("list siswa aktif: %w", err)
	}

	result := &ProcessResult{Total: len(siswaList)}
	sem := make(chan struct{}, workerPoolSize)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, siswa := range siswaList {
		sem <- struct{}{}
		wg.Add(1)
		go func(sw model.MstSiswa) {
			defer wg.Done()
			defer func() { <-sem }()

			if err := s.checkAndUpsert(ctx, sw.ID); err != nil {
				log.Printf("ews check failed for siswa %d: %v", sw.ID, err)
				mu.Lock()
				result.Errors++
				mu.Unlock()
			} else {
				mu.Lock()
				result.Processed++
				mu.Unlock()
			}
		}(siswa)
	}
	wg.Wait()
	return result, nil
}

// ProcessSiswa runs the EWS check for a single student.
func (s *EWSService) ProcessSiswa(ctx context.Context, siswaID int64) error {
	if _, err := s.siswaRepo.FindByID(ctx, siswaID); err != nil {
		return repository.ErrNotFound
	}
	return s.checkAndUpsert(ctx, siswaID)
}

// checkAndUpsert evaluates all three EWS indicators and upserts/resolves alerts accordingly.
func (s *EWSService) checkAndUpsert(ctx context.Context, siswaID int64) error {
	// --- Indicator 1: Absensi alpha ≥ 3 in last 30 days ---
	absCount, err := s.ewsRepo.CountAbsensiAlpha(ctx, siswaID, 30)
	if err != nil {
		return fmt.Errorf("absensi check: %w", err)
	}
	if absCount >= 3 {
		level := absensiLevel(absCount)
		pesan := fmt.Sprintf("Siswa telah absent %d hari dalam sebulan", absCount)
		if err := s.ewsRepo.UpsertAlert(ctx, siswaID, "absensi", level, pesan); err != nil {
			return fmt.Errorf("upsert absensi alert: %w", err)
		}
	} else {
		_ = s.ewsRepo.AutoResolve(ctx, siswaID, "absensi")
	}

	// --- Indicator 2: Rata-rata nilai < 70 in last 90 days ---
	avgNilai, err := s.ewsRepo.AvgNilai(ctx, siswaID, 90)
	if err != nil {
		return fmt.Errorf("nilai check: %w", err)
	}
	if avgNilai >= 0 && avgNilai < 70 {
		level := nilaiLevel(avgNilai)
		pesan := fmt.Sprintf("Rata-rata nilai siswa di bawah standar: %.2f", avgNilai)
		if err := s.ewsRepo.UpsertAlert(ctx, siswaID, "nilai", level, pesan); err != nil {
			return fmt.Errorf("upsert nilai alert: %w", err)
		}
	} else {
		_ = s.ewsRepo.AutoResolve(ctx, siswaID, "nilai")
	}

	// --- Indicator 3: BK cases in last 30 days ---
	bkCount, err := s.ewsRepo.CountBKKasus(ctx, siswaID, 30)
	if err != nil {
		return fmt.Errorf("bk check: %w", err)
	}
	if bkCount > 0 {
		level := bkLevel(bkCount)
		pesan := fmt.Sprintf("Siswa memiliki %d catatan perilaku dalam sebulan", bkCount)
		if err := s.ewsRepo.UpsertAlert(ctx, siswaID, "perilaku", level, pesan); err != nil {
			return fmt.Errorf("upsert perilaku alert: %w", err)
		}
	} else {
		_ = s.ewsRepo.AutoResolve(ctx, siswaID, "perilaku")
	}

	return nil
}

func absensiLevel(count int) int32 {
	if count >= 5 {
		return 3
	}
	if count >= 4 {
		return 2
	}
	return 1
}

func nilaiLevel(avg float64) int32 {
	if avg < 50 {
		return 3
	}
	if avg < 60 {
		return 2
	}
	return 1
}

func bkLevel(count int) int32 {
	if count >= 3 {
		return 3
	}
	if count >= 2 {
		return 2
	}
	return 1
}

// ListAlerts returns paginated EWS alerts with optional filters.
func (s *EWSService) ListAlerts(ctx context.Context, filters map[string]string, page, perPage int) ([]model.TrxEWSAlert, int, error) {
	return s.ewsRepo.ListAlerts(ctx, filters, page, perPage)
}

// GetAlertsBySiswa returns all EWS alerts for a specific student.
func (s *EWSService) GetAlertsBySiswa(ctx context.Context, siswaID int64) ([]model.TrxEWSAlert, error) {
	return s.ewsRepo.GetAlertsBySiswa(ctx, siswaID)
}

// ResolveAlert resolves a specific EWS alert by ID.
func (s *EWSService) ResolveAlert(ctx context.Context, id int64) error {
	return s.ewsRepo.ResolveByID(ctx, id)
}
