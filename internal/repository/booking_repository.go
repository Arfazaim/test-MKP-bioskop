package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"bioskop-mkp/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrBookingNotFound      = errors.New("pesanan tidak ditemukan")
	ErrSeatAlreadyTaken     = errors.New("salah satu kursi baru saja dipilih pengguna lain, silakan pilih kursi lain")
	ErrInvalidSeats         = errors.New("kursi tidak valid atau bukan milik studio jadwal ini")
	ErrScheduleNotAvailable = errors.New("jadwal tayang ini sudah tidak melayani pemesanan")
	ErrBookingExpired       = errors.New("waktu pembayaran 10 menit telah habis dan kursi sudah dilepas kembali")
	ErrBookingNotPending    = errors.New("status pesanan ini tidak dapat diproses lagi")
	ErrShowAlreadyStarted   = errors.New("tiket tidak dapat dibatalkan karena jam tayang sudah dimulai")
)

type BookingRepository interface {
	ListCatalogMetadata(ctx context.Context) ([]model.Movie, []model.Cinema, []model.Studio, []model.City, error)
	GetScheduleSeats(ctx context.Context, scheduleID int64) (*model.Schedule, []model.SeatAvailability, error)
	CreateBooking(ctx context.Context, userID, scheduleID int64, seatIDs []int64) (*model.Booking, error)
	ListUserBookings(ctx context.Context, userID int64) ([]model.Booking, error)
	FindBookingByID(ctx context.Context, id int64) (*model.Booking, error)
	PayBooking(ctx context.Context, bookingID, userID int64, method string, discount float64) (*model.Booking, error)
	CancelBooking(ctx context.Context, bookingID, userID int64) (*model.Booking, error)
	ExpireOverdueBookings(ctx context.Context) (int64, error)
}

type bookingRepository struct {
	db *gorm.DB
}

func NewBookingRepository(db *gorm.DB) BookingRepository {
	return &bookingRepository{db: db}
}

func preloadBooking(db *gorm.DB) *gorm.DB {
	return db.
		Preload("Schedule").
		Preload("Schedule.Movie").
		Preload("Schedule.Studio").
		Preload("Schedule.Studio.Cinema").
		Preload("Schedule.Studio.Cinema.City").
		Preload("Seats").
		Preload("Seats.Seat").
		Preload("Payment").
		Preload("Refund")
}

func (r *bookingRepository) ListCatalogMetadata(ctx context.Context) ([]model.Movie, []model.Cinema, []model.Studio, []model.City, error) {
	var movies []model.Movie
	if err := r.db.WithContext(ctx).Order("id ASC").Find(&movies).Error; err != nil {
		return nil, nil, nil, nil, err
	}
	var cities []model.City
	if err := r.db.WithContext(ctx).Order("name ASC").Find(&cities).Error; err != nil {
		return nil, nil, nil, nil, err
	}
	var cinemas []model.Cinema
	if err := r.db.WithContext(ctx).Preload("City").Order("id ASC").Find(&cinemas).Error; err != nil {
		return nil, nil, nil, nil, err
	}
	var studios []model.Studio
	if err := r.db.WithContext(ctx).Preload("Cinema").Order("cinema_id ASC, id ASC").Find(&studios).Error; err != nil {
		return nil, nil, nil, nil, err
	}
	return movies, cinemas, studios, cities, nil
}

func (r *bookingRepository) GetScheduleSeats(ctx context.Context, scheduleID int64) (*model.Schedule, []model.SeatAvailability, error) {
	_, _ = r.ExpireOverdueBookings(ctx)

	var schedule model.Schedule
	if err := preloadSchedule(r.db.WithContext(ctx)).First(&schedule, scheduleID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrScheduleNotFound
		}
		return nil, nil, err
	}

	var seats []model.Seat
	if err := r.db.WithContext(ctx).
		Where("studio_id = ?", schedule.StudioID).
		Order("row_label ASC, seat_number ASC").
		Find(&seats).Error; err != nil {
		return nil, nil, err
	}

	var activeBookingSeats []model.BookingSeat
	if err := r.db.WithContext(ctx).
		Where("schedule_id = ? AND status IN ?", scheduleID, []string{model.SeatStatusLocked, model.SeatStatusBooked}).
		Find(&activeBookingSeats).Error; err != nil {
		return nil, nil, err
	}

	statusBySeat := make(map[int64]string, len(activeBookingSeats))
	for _, bs := range activeBookingSeats {
		statusBySeat[bs.SeatID] = bs.Status
	}

	out := make([]model.SeatAvailability, 0, len(seats))
	for _, s := range seats {
		st := "AVAILABLE"
		if active, ok := statusBySeat[s.ID]; ok {
			st = active
		}
		out = append(out, model.SeatAvailability{
			ID:         s.ID,
			StudioID:   s.StudioID,
			RowLabel:   s.RowLabel,
			SeatNumber: s.SeatNumber,
			SeatType:   s.SeatType,
			Status:     st,
		})
	}
	return &schedule, out, nil
}

func (r *bookingRepository) CreateBooking(ctx context.Context, userID, scheduleID int64, seatIDs []int64) (*model.Booking, error) {
	_, _ = r.ExpireOverdueBookings(ctx)

	uniqueIDs := make([]int64, 0, len(seatIDs))
	seen := make(map[int64]bool, len(seatIDs))
	for _, id := range seatIDs {
		if id > 0 && !seen[id] {
			seen[id] = true
			uniqueIDs = append(uniqueIDs, id)
		}
	}
	if len(uniqueIDs) == 0 {
		return nil, ErrInvalidSeats
	}
	// Sort ascending to prevent deadlocks when concurrent buyers select overlapping seat sets.
	slices.Sort(uniqueIDs)

	var created model.Booking
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var schedule model.Schedule
		if err := tx.First(&schedule, scheduleID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrScheduleNotFound
			}
			return err
		}
		if schedule.Status != model.ScheduleStatusScheduled {
			return ErrScheduleNotAvailable
		}

		var seats []model.Seat
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id IN ? AND studio_id = ?", uniqueIDs, schedule.StudioID).
			Order("id ASC").
			Find(&seats).Error; err != nil {
			return err
		}
		if len(seats) != len(uniqueIDs) {
			return ErrInvalidSeats
		}

		booking := model.Booking{
			BookingCode: generateBookingCode(),
			UserID:      userID,
			ScheduleID:  schedule.ID,
			TotalAmount: schedule.Price * float64(len(seats)),
			Status:      model.BookingStatusPending,
			ExpiredAt:   time.Now().Add(10 * time.Minute),
		}
		if err := tx.Create(&booking).Error; err != nil {
			return err
		}

		bookingSeats := make([]model.BookingSeat, 0, len(seats))
		for _, seat := range seats {
			bookingSeats = append(bookingSeats, model.BookingSeat{
				BookingID:  booking.ID,
				ScheduleID: schedule.ID,
				SeatID:     seat.ID,
				Price:      schedule.Price,
				Status:     model.SeatStatusLocked,
			})
		}
		if err := tx.Create(&bookingSeats).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return ErrSeatAlreadyTaken
			}
			return err
		}

		return preloadBooking(tx).First(&created, booking.ID).Error
	})
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func (r *bookingRepository) ListUserBookings(ctx context.Context, userID int64) ([]model.Booking, error) {
	_, _ = r.ExpireOverdueBookings(ctx)

	var bookings []model.Booking
	if err := preloadBooking(r.db.WithContext(ctx)).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&bookings).Error; err != nil {
		return nil, err
	}
	return bookings, nil
}

func (r *bookingRepository) FindBookingByID(ctx context.Context, id int64) (*model.Booking, error) {
	_, _ = r.ExpireOverdueBookings(ctx)

	var booking model.Booking
	if err := preloadBooking(r.db.WithContext(ctx)).First(&booking, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBookingNotFound
		}
		return nil, err
	}
	return &booking, nil
}

func (r *bookingRepository) PayBooking(ctx context.Context, bookingID, userID int64, method string, discount float64) (*model.Booking, error) {
	var updated model.Booking
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var booking model.Booking
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", bookingID, userID).
			First(&booking).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrBookingNotFound
			}
			return err
		}

		now := time.Now()
		if booking.Status == model.BookingStatusPending && !booking.ExpiredAt.After(now) {
			_ = tx.Model(&booking).Updates(map[string]any{"status": model.BookingStatusExpired, "updated_at": now}).Error
			_ = tx.Model(&model.BookingSeat{}).Where("booking_id = ?", booking.ID).Update("status", model.SeatStatusReleased).Error
			return ErrBookingExpired
		}
		if booking.Status != model.BookingStatusPending {
			return ErrBookingNotPending
		}

		serviceFee := 4000.0
		finalAmount := booking.TotalAmount + serviceFee - discount
		if finalAmount < 0 {
			finalAmount = 0
		}

		ref := fmt.Sprintf("PAY-%d-%s", booking.ID, strings.ToUpper(randomHex(3)))
		payment := model.Payment{
			BookingID:        booking.ID,
			PaymentMethod:    strings.ToUpper(strings.TrimSpace(method)),
			GatewayReference: &ref,
			Amount:           finalAmount,
			Status:           "SUCCESS",
			PaidAt:           &now,
		}
		if err := tx.Create(&payment).Error; err != nil {
			return err
		}

		if err := tx.Model(&booking).Updates(map[string]any{
			"total_amount": finalAmount,
			"status":       model.BookingStatusPaid,
			"updated_at":   now,
		}).Error; err != nil {
			return err
		}

		if err := tx.Model(&model.BookingSeat{}).
			Where("booking_id = ?", booking.ID).
			Update("status", model.SeatStatusBooked).Error; err != nil {
			return err
		}

		return preloadBooking(tx).First(&updated, booking.ID).Error
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func (r *bookingRepository) CancelBooking(ctx context.Context, bookingID, userID int64) (*model.Booking, error) {
	var updated model.Booking
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var booking model.Booking
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ?", bookingID, userID).
			First(&booking).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrBookingNotFound
			}
			return err
		}

		now := time.Now()
		switch booking.Status {
		case model.BookingStatusPending:
			if err := tx.Model(&booking).Updates(map[string]any{
				"status":     model.BookingStatusCancelled,
				"updated_at": now,
			}).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.BookingSeat{}).
				Where("booking_id = ?", booking.ID).
				Update("status", model.SeatStatusReleased).Error; err != nil {
				return err
			}

		case model.BookingStatusPaid:
			var schedule model.Schedule
			if err := tx.First(&schedule, booking.ScheduleID).Error; err != nil {
				return err
			}
			if !schedule.StartTime.After(now) {
				return ErrShowAlreadyStarted
			}

			var payment model.Payment
			if err := tx.Where("booking_id = ?", booking.ID).First(&payment).Error; err == nil {
				refund := model.Refund{
					BookingID:      booking.ID,
					PaymentID:      payment.ID,
					IdempotencyKey: "CUSTOMER_CANCEL_BOOKING_" + booking.BookingCode,
					RefundAmount:   payment.Amount,
					Reason:         "Pembatalan tiket oleh pelanggan sebelum jam tayang",
					InitiatedBy:    "CUSTOMER",
					Status:         "SUCCESS",
					RefundedAt:     &now,
				}
				if err := tx.Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "idempotency_key"}},
					DoNothing: true,
				}).Create(&refund).Error; err != nil {
					return err
				}
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}

			if err := tx.Model(&booking).Updates(map[string]any{
				"status":     model.BookingStatusRefunded,
				"updated_at": now,
			}).Error; err != nil {
				return err
			}
			// Setting booking_seats to REFUNDED releases uq_active_schedule_seat so seats restock immediately.
			if err := tx.Model(&model.BookingSeat{}).
				Where("booking_id = ?", booking.ID).
				Update("status", model.SeatStatusRefunded).Error; err != nil {
				return err
			}

		default:
			return ErrBookingNotPending
		}

		return preloadBooking(tx).First(&updated, booking.ID).Error
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func (r *bookingRepository) ExpireOverdueBookings(ctx context.Context) (int64, error) {
	var expiredCount int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var overdue []model.Booking
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND expired_at <= ?", model.BookingStatusPending, time.Now()).
			Limit(200).
			Find(&overdue).Error; err != nil {
			return err
		}
		if len(overdue) == 0 {
			return nil
		}

		ids := make([]int64, 0, len(overdue))
		for _, b := range overdue {
			ids = append(ids, b.ID)
		}
		now := time.Now()
		if err := tx.Model(&model.Booking{}).Where("id IN ?", ids).
			Updates(map[string]any{"status": model.BookingStatusExpired, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.BookingSeat{}).Where("booking_id IN ?", ids).
			Update("status", model.SeatStatusReleased).Error; err != nil {
			return err
		}
		expiredCount = int64(len(ids))
		return nil
	})
	return expiredCount, err
}

func generateBookingCode() string {
	return "ZNM-" + strings.ToUpper(randomHex(4))
}

func randomHex(nBytes int) string {
	b := make([]byte, nBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

