package repository

import (
	"context"
	"errors"
	"time"

	"bioskop-mkp/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrScheduleNotFound    = errors.New("jadwal tayang tidak ditemukan")
	ErrMovieNotFound       = errors.New("film tidak ditemukan")
	ErrStudioNotFound      = errors.New("studio tidak ditemukan")
	ErrScheduleOverlap     = errors.New("jadwal bentrok dengan jadwal lain di studio yang sama")
	ErrScheduleHasBookings = errors.New("jadwal tidak bisa dihapus karena sudah memiliki pesanan")
)

type ScheduleRepository interface {
	FindAll(ctx context.Context, filter model.ScheduleFilter) ([]model.Schedule, int64, error)
	FindByID(ctx context.Context, id int64) (*model.Schedule, error)
	Create(ctx context.Context, schedule *model.Schedule, autoEndTime bool) error
	Update(ctx context.Context, id int64, mutate func(existing *model.Schedule) error) (*model.Schedule, error)
	Delete(ctx context.Context, id int64) error
}

type scheduleRepository struct {
	db *gorm.DB
}

func NewScheduleRepository(db *gorm.DB) ScheduleRepository {
	return &scheduleRepository{db: db}
}

func preloadSchedule(db *gorm.DB) *gorm.DB {
	return db.Preload("Movie").Preload("Studio").Preload("Studio.Cinema").Preload("Studio.Cinema.City")
}

func (r *scheduleRepository) FindAll(ctx context.Context, filter model.ScheduleFilter) ([]model.Schedule, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.Schedule{})

	if filter.MovieID != nil {
		q = q.Where("schedules.movie_id = ?", *filter.MovieID)
	}
	if filter.StudioID != nil {
		q = q.Where("schedules.studio_id = ?", *filter.StudioID)
	}
	if filter.CinemaID != nil {
		q = q.Joins("JOIN studios ON studios.id = schedules.studio_id").
			Where("studios.cinema_id = ?", *filter.CinemaID)
	}
	if filter.Status != "" {
		q = q.Where("schedules.status = ?", filter.Status)
	}
	if filter.Date != "" {
		loc := time.FixedZone("WIB", 7*3600)
		dayStart, err := time.ParseInLocation("2006-01-02", filter.Date, loc)
		if err != nil {
			return nil, 0, err
		}
		dayEnd := dayStart.Add(24 * time.Hour)
		q = q.Where("schedules.start_time >= ? AND schedules.start_time < ?", dayStart, dayEnd)
	}

	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var schedules []model.Schedule
	offset := (filter.Page - 1) * filter.Limit
	if err := preloadSchedule(q.Session(&gorm.Session{})).
		Order("schedules.start_time ASC, schedules.id ASC").
		Offset(offset).
		Limit(filter.Limit).
		Find(&schedules).Error; err != nil {
		return nil, 0, err
	}

	return schedules, total, nil
}

func (r *scheduleRepository) FindByID(ctx context.Context, id int64) (*model.Schedule, error) {
	var schedule model.Schedule
	if err := preloadSchedule(r.db.WithContext(ctx)).First(&schedule, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrScheduleNotFound
		}
		return nil, err
	}
	return &schedule, nil
}

func (r *scheduleRepository) Create(ctx context.Context, schedule *model.Schedule, autoEndTime bool) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock studio row to serialize overlap checks for concurrent schedule creation in the same studio.
		var studio model.Studio
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&studio, schedule.StudioID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrStudioNotFound
			}
			return err
		}

		var movie model.Movie
		if err := tx.First(&movie, schedule.MovieID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrMovieNotFound
			}
			return err
		}

		if autoEndTime {
			schedule.EndTime = schedule.StartTime.Add(time.Duration(movie.DurationMinutes) * time.Minute)
		}

		if err := ensureNoOverlap(tx, schedule.StudioID, schedule.StartTime, schedule.EndTime, 0); err != nil {
			return err
		}

		if err := tx.Create(schedule).Error; err != nil {
			return err
		}

		return preloadSchedule(tx).First(schedule, schedule.ID).Error
	})
}

func (r *scheduleRepository) Update(ctx context.Context, id int64, mutate func(existing *model.Schedule) error) (*model.Schedule, error) {
	var updated model.Schedule
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&updated, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrScheduleNotFound
			}
			return err
		}

		if err := mutate(&updated); err != nil {
			return err
		}

		var studio model.Studio
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&studio, updated.StudioID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrStudioNotFound
			}
			return err
		}

		var movie model.Movie
		if err := tx.First(&movie, updated.MovieID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrMovieNotFound
			}
			return err
		}

		if updated.Status != model.ScheduleStatusCancelled {
			if err := ensureNoOverlap(tx, updated.StudioID, updated.StartTime, updated.EndTime, updated.ID); err != nil {
				return err
			}
		}

		if err := tx.Model(&model.Schedule{}).Where("id = ?", updated.ID).Updates(map[string]any{
			"movie_id":            updated.MovieID,
			"studio_id":           updated.StudioID,
			"start_time":          updated.StartTime,
			"end_time":            updated.EndTime,
			"price":               updated.Price,
			"status":              updated.Status,
			"cancellation_reason": updated.CancellationReason,
			"updated_at":          time.Now(),
		}).Error; err != nil {
			return err
		}

		if updated.Status == model.ScheduleStatusCancelled {
			reason := "jadwal dibatalkan pihak bioskop"
			if updated.CancellationReason != nil && *updated.CancellationReason != "" {
				reason = *updated.CancellationReason
			}
			if err := cancelAndRefundScheduleBookings(tx, updated.ID, reason); err != nil {
				return err
			}
		}

		return preloadSchedule(tx).First(&updated, updated.ID).Error
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func (r *scheduleRepository) Delete(ctx context.Context, id int64) error {
	res := r.db.WithContext(ctx).Delete(&model.Schedule{}, id)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrForeignKeyViolated) {
			return ErrScheduleHasBookings
		}
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrScheduleNotFound
	}
	return nil
}

func ensureNoOverlap(tx *gorm.DB, studioID int64, startTime, endTime time.Time, excludeID int64) error {
	q := tx.Model(&model.Schedule{}).
		Where("studio_id = ? AND status <> ? AND start_time < ? AND end_time > ?",
			studioID, model.ScheduleStatusCancelled, endTime, startTime)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrScheduleOverlap
	}
	return nil
}

func cancelAndRefundScheduleBookings(tx *gorm.DB, scheduleID int64, reason string) error {
	now := time.Now()

	// Cancel PENDING bookings and release their locked seats.
	var pendingIDs []int64
	if err := tx.Model(&model.Booking{}).
		Where("schedule_id = ? AND status = ?", scheduleID, model.BookingStatusPending).
		Pluck("id", &pendingIDs).Error; err != nil {
		return err
	}
	if len(pendingIDs) > 0 {
		if err := tx.Model(&model.Booking{}).Where("id IN ?", pendingIDs).
			Updates(map[string]any{"status": model.BookingStatusCancelled, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.BookingSeat{}).Where("booking_id IN ?", pendingIDs).
			Update("status", model.SeatStatusReleased).Error; err != nil {
			return err
		}
	}

	// Refund PAID bookings idempotently (100% full refund initiated by CINEMA).
	var paidBookings []model.Booking
	if err := tx.Preload("Payment").
		Where("schedule_id = ? AND status = ?", scheduleID, model.BookingStatusPaid).
		Find(&paidBookings).Error; err != nil {
		return err
	}
	for _, b := range paidBookings {
		if b.Payment != nil {
			refund := model.Refund{
				BookingID:      b.ID,
				PaymentID:      b.Payment.ID,
				IdempotencyKey: "CINEMA_CANCEL_BOOKING_" + b.BookingCode,
				RefundAmount:   b.Payment.Amount,
				Reason:         reason,
				InitiatedBy:    "CINEMA",
				Status:         "SUCCESS",
				RefundedAt:     &now,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "idempotency_key"}},
				DoNothing: true,
			}).Create(&refund).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&model.Booking{}).Where("id = ?", b.ID).
			Updates(map[string]any{"status": model.BookingStatusRefunded, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.BookingSeat{}).Where("booking_id = ?", b.ID).
			Update("status", model.SeatStatusRefunded).Error; err != nil {
			return err
		}
	}
	return nil
}

