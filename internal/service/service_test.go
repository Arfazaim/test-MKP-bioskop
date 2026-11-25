package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bioskop-mkp/internal/middleware"
	"bioskop-mkp/internal/model"
	"bioskop-mkp/internal/repository"

	"github.com/gin-gonic/gin"
)

type stubUserRepo struct {
	users  map[string]*model.User
	nextID int64
}

func newStubUserRepo() *stubUserRepo {
	return &stubUserRepo{users: make(map[string]*model.User), nextID: 1}
}

func (r *stubUserRepo) Create(_ context.Context, user *model.User) error {
	if _, ok := r.users[user.Email]; ok {
		return repository.ErrEmailAlreadyExists
	}
	user.ID = r.nextID
	r.nextID++
	cp := *user
	r.users[user.Email] = &cp
	return nil
}

func (r *stubUserRepo) FindByEmail(_ context.Context, email string) (*model.User, error) {
	u, ok := r.users[email]
	if !ok {
		return nil, repository.ErrUserNotFound
	}
	cp := *u
	return &cp, nil
}

func (r *stubUserRepo) FindByID(_ context.Context, id int64) (*model.User, error) {
	for _, u := range r.users {
		if u.ID == id {
			cp := *u
			return &cp, nil
		}
	}
	return nil, repository.ErrUserNotFound
}

type stubScheduleRepo struct {
	schedules map[int64]*model.Schedule
	nextID    int64
}

func newStubScheduleRepo() *stubScheduleRepo {
	return &stubScheduleRepo{schedules: make(map[int64]*model.Schedule), nextID: 1}
}

func (r *stubScheduleRepo) FindAll(_ context.Context, _ model.ScheduleFilter) ([]model.Schedule, int64, error) {
	out := make([]model.Schedule, 0, len(r.schedules))
	for _, s := range r.schedules {
		out = append(out, *s)
	}
	return out, int64(len(out)), nil
}

func (r *stubScheduleRepo) FindByID(_ context.Context, id int64) (*model.Schedule, error) {
	s, ok := r.schedules[id]
	if !ok {
		return nil, repository.ErrScheduleNotFound
	}
	cp := *s
	return &cp, nil
}

func (r *stubScheduleRepo) Create(_ context.Context, schedule *model.Schedule, autoEndTime bool) error {
	if autoEndTime {
		schedule.EndTime = schedule.StartTime.Add(120 * time.Minute)
	}
	for _, existing := range r.schedules {
		if existing.StudioID == schedule.StudioID &&
			existing.Status != model.ScheduleStatusCancelled &&
			existing.StartTime.Before(schedule.EndTime) &&
			existing.EndTime.After(schedule.StartTime) {
			return repository.ErrScheduleOverlap
		}
	}
	schedule.ID = r.nextID
	r.nextID++
	cp := *schedule
	r.schedules[schedule.ID] = &cp
	return nil
}

func (r *stubScheduleRepo) Update(_ context.Context, id int64, mutate func(existing *model.Schedule) error) (*model.Schedule, error) {
	s, ok := r.schedules[id]
	if !ok {
		return nil, repository.ErrScheduleNotFound
	}
	cp := *s
	if err := mutate(&cp); err != nil {
		return nil, err
	}
	r.schedules[id] = &cp
	return &cp, nil
}

func (r *stubScheduleRepo) Delete(_ context.Context, id int64) error {
	if _, ok := r.schedules[id]; !ok {
		return repository.ErrScheduleNotFound
	}
	delete(r.schedules, id)
	return nil
}

func TestAuthRegisterLoginAndMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	svc := NewAuthService(newStubUserRepo(), "test-secret", 1)

	user, err := svc.Register(ctx, model.RegisterRequest{
		FullName: "  Budi Santoso ",
		Email:    "Budi@Example.COM ",
		Password: "secret123",
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if user.Role != model.RoleCustomer || user.Email != "budi@example.com" {
		t.Fatalf("unexpected registered user: %+v", user)
	}

	if _, err := svc.Register(ctx, model.RegisterRequest{
		FullName: "Duplicate",
		Email:    "budi@example.com",
		Password: "secret123",
	}); !errors.Is(err, repository.ErrEmailAlreadyExists) {
		t.Fatalf("expected ErrEmailAlreadyExists, got %v", err)
	}

	if _, err := svc.Login(ctx, model.LoginRequest{
		Email:    "budi@example.com",
		Password: "wrong",
	}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}

	authResp, err := svc.Login(ctx, model.LoginRequest{
		Email:    "budi@example.com",
		Password: "secret123",
	})
	if err != nil || authResp.Token == "" {
		t.Fatalf("login failed: %v", err)
	}

	r := gin.New()
	r.GET("/admin", middleware.AuthMiddleware("test-secret"), middleware.RequireRole(model.RoleAdmin), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	r.GET("/me", middleware.AuthMiddleware("test-secret"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	reqMe := httptest.NewRequest(http.MethodGet, "/me", nil)
	reqMe.Header.Set("Authorization", "Bearer "+authResp.Token)
	wMe := httptest.NewRecorder()
	r.ServeHTTP(wMe, reqMe)
	if wMe.Code != http.StatusOK {
		t.Fatalf("expected 200 on /me, got %d", wMe.Code)
	}

	reqAdmin := httptest.NewRequest(http.MethodGet, "/admin", nil)
	reqAdmin.Header.Set("Authorization", "Bearer "+authResp.Token)
	wAdmin := httptest.NewRecorder()
	r.ServeHTTP(wAdmin, reqAdmin)
	if wAdmin.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for CUSTOMER on /admin, got %d", wAdmin.Code)
	}
}

func TestScheduleValidationAndCancel(t *testing.T) {
	ctx := context.Background()
	svc := NewScheduleService(newStubScheduleRepo())

	start := time.Date(2026, 12, 1, 14, 0, 0, 0, time.UTC)
	invalidEnd := start.Add(-10 * time.Minute)
	price := 50000.0

	if _, err := svc.Create(ctx, model.CreateScheduleRequest{
		MovieID:   1,
		StudioID:  1,
		StartTime: start,
		EndTime:   &invalidEnd,
		Price:     &price,
	}); !errors.Is(err, ErrInvalidTimeRange) {
		t.Fatalf("expected ErrInvalidTimeRange, got %v", err)
	}

	created, err := svc.Create(ctx, model.CreateScheduleRequest{
		MovieID:   1,
		StudioID:  1,
		StartTime: start,
		Price:     &price,
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	if _, err := svc.Create(ctx, model.CreateScheduleRequest{
		MovieID:   2,
		StudioID:  1,
		StartTime: start.Add(30 * time.Minute),
		Price:     &price,
	}); !errors.Is(err, repository.ErrScheduleOverlap) {
		t.Fatalf("expected ErrScheduleOverlap, got %v", err)
	}

	if _, err := svc.Cancel(ctx, created.ID, "   "); !errors.Is(err, ErrCancellationReasonRequired) {
		t.Fatalf("expected ErrCancellationReasonRequired, got %v", err)
	}

	cancelled, err := svc.Cancel(ctx, created.ID, "sedang ada kendala")
	if err != nil || cancelled.Status != model.ScheduleStatusCancelled {
		t.Fatalf("cancel failed: %v, %+v", err, cancelled)
	}

	if _, err := svc.Cancel(ctx, created.ID, "Again"); !errors.Is(err, ErrInvalidScheduleState) {
		t.Fatalf("expected ErrInvalidScheduleState on double cancel, got %v", err)
	}
}

type stubBookingRepo struct {
	lockedSeats map[int64]bool
	bookings    map[int64]*model.Booking
	nextID      int64
}

func newStubBookingRepo() *stubBookingRepo {
	return &stubBookingRepo{
		lockedSeats: make(map[int64]bool),
		bookings:    make(map[int64]*model.Booking),
		nextID:      1,
	}
}

func (r *stubBookingRepo) ListCatalogMetadata(_ context.Context) ([]model.Movie, []model.Cinema, []model.Studio, []model.City, error) {
	return nil, nil, nil, nil, nil
}

func (r *stubBookingRepo) GetScheduleSeats(_ context.Context, _ int64) (*model.Schedule, []model.SeatAvailability, error) {
	return &model.Schedule{ID: 1}, nil, nil
}

func (r *stubBookingRepo) CreateBooking(_ context.Context, userID, scheduleID int64, seatIDs []int64) (*model.Booking, error) {
	for _, id := range seatIDs {
		if r.lockedSeats[id] {
			return nil, repository.ErrSeatAlreadyTaken
		}
	}
	for _, id := range seatIDs {
		r.lockedSeats[id] = true
	}
	b := &model.Booking{
		ID:          r.nextID,
		BookingCode: "CINE-TEST",
		UserID:      userID,
		ScheduleID:  scheduleID,
		TotalAmount: float64(len(seatIDs)) * 50000,
		Status:      model.BookingStatusPending,
		ExpiredAt:   time.Now().Add(10 * time.Minute),
	}
	r.nextID++
	r.bookings[b.ID] = b
	return b, nil
}

func (r *stubBookingRepo) ListUserBookings(_ context.Context, _ int64) ([]model.Booking, error) {
	return nil, nil
}

func (r *stubBookingRepo) FindBookingByID(_ context.Context, id int64) (*model.Booking, error) {
	b, ok := r.bookings[id]
	if !ok {
		return nil, repository.ErrBookingNotFound
	}
	return b, nil
}

func (r *stubBookingRepo) PayBooking(_ context.Context, bookingID, _ int64, _ string, discount float64) (*model.Booking, error) {
	b, ok := r.bookings[bookingID]
	if !ok {
		return nil, repository.ErrBookingNotFound
	}
	b.TotalAmount = b.TotalAmount + 4000 - discount
	b.Status = model.BookingStatusPaid
	return b, nil
}

func (r *stubBookingRepo) CancelBooking(_ context.Context, bookingID, _ int64) (*model.Booking, error) {
	b, ok := r.bookings[bookingID]
	if !ok {
		return nil, repository.ErrBookingNotFound
	}
	switch b.Status {
	case model.BookingStatusPending:
		b.Status = model.BookingStatusCancelled
	case model.BookingStatusPaid:
		b.Status = model.BookingStatusRefunded
		b.Refund = &model.Refund{
			BookingID:    b.ID,
			RefundAmount: b.TotalAmount,
			InitiatedBy:  "CUSTOMER",
			Status:       "SUCCESS",
		}
	default:
		return nil, repository.ErrBookingNotPending
	}
	for id := range r.lockedSeats {
		delete(r.lockedSeats, id)
	}
	return b, nil
}

func (r *stubBookingRepo) ExpireOverdueBookings(_ context.Context) (int64, error) {
	return 0, nil
}

func TestBookingLockAndPay(t *testing.T) {
	ctx := context.Background()
	svc := NewBookingService(newStubBookingRepo())

	booking, err := svc.CreateBooking(ctx, 4, model.CreateBookingRequest{
		ScheduleID: 1,
		SeatIDs:    []int64{1, 2},
	})
	if err != nil {
		t.Fatalf("create booking failed: %v", err)
	}

	if _, err := svc.CreateBooking(ctx, 5, model.CreateBookingRequest{
		ScheduleID: 1,
		SeatIDs:    []int64{2, 3},
	}); !errors.Is(err, repository.ErrSeatAlreadyTaken) {
		t.Fatalf("expected ErrSeatAlreadyTaken on overlapping seat, got %v", err)
	}

	if _, err := svc.PayBooking(ctx, booking.ID, 4, model.PayBookingRequest{
		PaymentMethod: "UNKNOWN",
	}); !errors.Is(err, ErrInvalidPaymentMethod) {
		t.Fatalf("expected ErrInvalidPaymentMethod, got %v", err)
	}

	paid, err := svc.PayBooking(ctx, booking.ID, 4, model.PayBookingRequest{
		PaymentMethod: "QRIS",
		PromoCode:     "DISKON25",
	})
	if err != nil || paid.Status != model.BookingStatusPaid || paid.TotalAmount != 79000 {
		t.Fatalf("unexpected paid booking: err=%v booking=%+v", err, paid)
	}

	refunded, err := svc.CancelBooking(ctx, paid.ID, 4)
	if err != nil || refunded.Status != model.BookingStatusRefunded || refunded.Refund == nil || refunded.Refund.InitiatedBy != "CUSTOMER" {
		t.Fatalf("expected customer refund on paid booking cancel, got err=%v booking=%+v", err, refunded)
	}

	rebooked, err := svc.CreateBooking(ctx, 5, model.CreateBookingRequest{
		ScheduleID: 1,
		SeatIDs:    []int64{1, 2},
	})
	if err != nil || rebooked.ID == 0 {
		t.Fatalf("expected seats to be restocked after paid cancel, got err=%v", err)
	}
}

