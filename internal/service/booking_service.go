package service

import (
	"context"
	"errors"
	"strings"

	"bioskop-mkp/internal/model"
	"bioskop-mkp/internal/repository"
)

var (
	ErrInvalidPaymentMethod = errors.New("metode pembayaran tidak valid")
	ErrForbiddenBooking     = errors.New("anda tidak memiliki akses ke pesanan ini")
)

type CatalogMetadata struct {
	Movies  []model.Movie  `json:"movies"`
	Cinemas []model.Cinema `json:"cinemas"`
	Studios []model.Studio `json:"studios"`
	Cities  []model.City   `json:"cities"`
}

type ScheduleSeatsResponse struct {
	Schedule *model.Schedule          `json:"schedule"`
	Seats    []model.SeatAvailability `json:"seats"`
}

type BookingService interface {
	GetCatalogMetadata(ctx context.Context) (*CatalogMetadata, error)
	GetScheduleSeats(ctx context.Context, scheduleID int64) (*ScheduleSeatsResponse, error)
	CreateBooking(ctx context.Context, userID int64, req model.CreateBookingRequest) (*model.Booking, error)
	ListMyBookings(ctx context.Context, userID int64) ([]model.Booking, error)
	GetBooking(ctx context.Context, bookingID, userID int64, role string) (*model.Booking, error)
	PayBooking(ctx context.Context, bookingID, userID int64, req model.PayBookingRequest) (*model.Booking, error)
	CancelBooking(ctx context.Context, bookingID, userID int64) (*model.Booking, error)
	ExpireOverdueBookings(ctx context.Context) (int64, error)
}

type bookingService struct {
	repo repository.BookingRepository
}

func NewBookingService(repo repository.BookingRepository) BookingService {
	return &bookingService{repo: repo}
}

func (s *bookingService) GetCatalogMetadata(ctx context.Context) (*CatalogMetadata, error) {
	movies, cinemas, studios, cities, err := s.repo.ListCatalogMetadata(ctx)
	if err != nil {
		return nil, err
	}
	return &CatalogMetadata{
		Movies:  movies,
		Cinemas: cinemas,
		Studios: studios,
		Cities:  cities,
	}, nil
}

func (s *bookingService) GetScheduleSeats(ctx context.Context, scheduleID int64) (*ScheduleSeatsResponse, error) {
	if scheduleID <= 0 {
		return nil, ErrInvalidInput
	}
	schedule, seats, err := s.repo.GetScheduleSeats(ctx, scheduleID)
	if err != nil {
		return nil, err
	}
	return &ScheduleSeatsResponse{Schedule: schedule, Seats: seats}, nil
}

func (s *bookingService) CreateBooking(ctx context.Context, userID int64, req model.CreateBookingRequest) (*model.Booking, error) {
	if userID <= 0 || req.ScheduleID <= 0 || len(req.SeatIDs) == 0 || len(req.SeatIDs) > 8 {
		return nil, ErrInvalidInput
	}
	return s.repo.CreateBooking(ctx, userID, req.ScheduleID, req.SeatIDs)
}

func (s *bookingService) ListMyBookings(ctx context.Context, userID int64) ([]model.Booking, error) {
	return s.repo.ListUserBookings(ctx, userID)
}

func (s *bookingService) GetBooking(ctx context.Context, bookingID, userID int64, role string) (*model.Booking, error) {
	booking, err := s.repo.FindBookingByID(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	if booking.UserID != userID && role != model.RoleAdmin {
		return nil, ErrForbiddenBooking
	}
	return booking, nil
}

func (s *bookingService) PayBooking(ctx context.Context, bookingID, userID int64, req model.PayBookingRequest) (*model.Booking, error) {
	method := strings.ToUpper(strings.TrimSpace(req.PaymentMethod))
	switch method {
	case "QRIS", "EWALLET", "VA", "CARD":
	default:
		return nil, ErrInvalidPaymentMethod
	}

	discount := 0.0
	if strings.EqualFold(strings.TrimSpace(req.PromoCode), "DISKON25") {
		discount = 25000.0
	}
	return s.repo.PayBooking(ctx, bookingID, userID, method, discount)
}

func (s *bookingService) CancelBooking(ctx context.Context, bookingID, userID int64) (*model.Booking, error) {
	return s.repo.CancelBooking(ctx, bookingID, userID)
}

func (s *bookingService) ExpireOverdueBookings(ctx context.Context) (int64, error) {
	return s.repo.ExpireOverdueBookings(ctx)
}

