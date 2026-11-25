package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"bioskop-mkp/internal/model"
	"bioskop-mkp/internal/repository"
)

var (
	ErrInvalidTimeRange           = errors.New("waktu selesai harus setelah waktu mulai")
	ErrInvalidStatus              = errors.New("status jadwal tidak valid")
	ErrInvalidScheduleState       = errors.New("jadwal yang sudah selesai atau dibatalkan tidak bisa diubah")
	ErrCancellationReasonRequired = errors.New("alasan pembatalan wajib diisi")
	ErrInvalidDateFilter          = errors.New("format tanggal harus YYYY-MM-DD")
	ErrInvalidPrice               = errors.New("harga tiket tidak boleh negatif")
)

type ScheduleService interface {
	List(ctx context.Context, filter model.ScheduleFilter) (*model.ScheduleListResponse, error)
	GetByID(ctx context.Context, id int64) (*model.Schedule, error)
	Create(ctx context.Context, req model.CreateScheduleRequest) (*model.Schedule, error)
	Update(ctx context.Context, id int64, req model.UpdateScheduleRequest) (*model.Schedule, error)
	Cancel(ctx context.Context, id int64, reason string) (*model.Schedule, error)
	Delete(ctx context.Context, id int64) error
}

type scheduleService struct {
	repo repository.ScheduleRepository
}

func NewScheduleService(repo repository.ScheduleRepository) ScheduleService {
	return &scheduleService{repo: repo}
}

func (s *scheduleService) List(ctx context.Context, filter model.ScheduleFilter) (*model.ScheduleListResponse, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.Limit <= 0 {
		filter.Limit = 20
	} else if filter.Limit > 100 {
		filter.Limit = 100
	}

	if filter.Status != "" {
		filter.Status = strings.ToUpper(strings.TrimSpace(filter.Status))
		if !model.IsValidScheduleStatus(filter.Status) {
			return nil, ErrInvalidStatus
		}
	}

	if filter.Date != "" {
		filter.Date = strings.TrimSpace(filter.Date)
		if _, err := time.Parse("2006-01-02", filter.Date); err != nil {
			return nil, ErrInvalidDateFilter
		}
	}

	items, total, err := s.repo.FindAll(ctx, filter)
	if err != nil {
		return nil, err
	}

	return &model.ScheduleListResponse{
		Items: items,
		Total: total,
		Page:  filter.Page,
		Limit: filter.Limit,
	}, nil
}

func (s *scheduleService) GetByID(ctx context.Context, id int64) (*model.Schedule, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *scheduleService) Create(ctx context.Context, req model.CreateScheduleRequest) (*model.Schedule, error) {
	if req.MovieID <= 0 || req.StudioID <= 0 || req.StartTime.IsZero() || req.Price == nil {
		return nil, ErrInvalidInput
	}
	if *req.Price < 0 {
		return nil, ErrInvalidPrice
	}

	autoEndTime := req.EndTime == nil
	var endTime time.Time
	if !autoEndTime {
		if !req.EndTime.After(req.StartTime) {
			return nil, ErrInvalidTimeRange
		}
		endTime = *req.EndTime
	}

	schedule := &model.Schedule{
		MovieID:   req.MovieID,
		StudioID:  req.StudioID,
		StartTime: req.StartTime,
		EndTime:   endTime,
		Price:     *req.Price,
		Status:    model.ScheduleStatusScheduled,
	}
	if err := s.repo.Create(ctx, schedule, autoEndTime); err != nil {
		return nil, err
	}
	return schedule, nil
}

func (s *scheduleService) Update(ctx context.Context, id int64, req model.UpdateScheduleRequest) (*model.Schedule, error) {
	return s.repo.Update(ctx, id, func(existing *model.Schedule) error {
		if existing.Status == model.ScheduleStatusCancelled || existing.Status == model.ScheduleStatusCompleted {
			return ErrInvalidScheduleState
		}

		if req.MovieID != nil {
			if *req.MovieID <= 0 {
				return ErrInvalidInput
			}
			existing.MovieID = *req.MovieID
		}
		if req.StudioID != nil {
			if *req.StudioID <= 0 {
				return ErrInvalidInput
			}
			existing.StudioID = *req.StudioID
		}
		if req.StartTime != nil {
			if req.StartTime.IsZero() {
				return ErrInvalidInput
			}
			existing.StartTime = *req.StartTime
		}
		if req.EndTime != nil {
			if req.EndTime.IsZero() {
				return ErrInvalidInput
			}
			existing.EndTime = *req.EndTime
		}
		if !existing.EndTime.After(existing.StartTime) {
			return ErrInvalidTimeRange
		}

		if req.Price != nil {
			if *req.Price < 0 {
				return ErrInvalidPrice
			}
			existing.Price = *req.Price
		}

		if req.Status != nil {
			status := strings.ToUpper(strings.TrimSpace(*req.Status))
			if !model.IsValidScheduleStatus(status) {
				return ErrInvalidStatus
			}
			existing.Status = status
		}

		if req.CancellationReason != nil {
			trimmed := strings.TrimSpace(*req.CancellationReason)
			if trimmed == "" {
				existing.CancellationReason = nil
			} else {
				existing.CancellationReason = &trimmed
			}
		}

		if existing.Status == model.ScheduleStatusCancelled && (existing.CancellationReason == nil || *existing.CancellationReason == "") {
			return ErrCancellationReasonRequired
		}

		return nil
	})
}

func (s *scheduleService) Cancel(ctx context.Context, id int64, reason string) (*model.Schedule, error) {
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return nil, ErrCancellationReasonRequired
	}

	return s.repo.Update(ctx, id, func(existing *model.Schedule) error {
		if existing.Status == model.ScheduleStatusCancelled || existing.Status == model.ScheduleStatusCompleted {
			return ErrInvalidScheduleState
		}
		existing.Status = model.ScheduleStatusCancelled
		existing.CancellationReason = &trimmed
		return nil
	})
}

func (s *scheduleService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

