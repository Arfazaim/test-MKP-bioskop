package model

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	RoleAdmin    = "ADMIN"
	RoleCustomer = "CUSTOMER"
)

const (
	ScheduleStatusScheduled = "SCHEDULED"
	ScheduleStatusOngoing   = "ONGOING"
	ScheduleStatusCompleted = "COMPLETED"
	ScheduleStatusCancelled = "CANCELLED"
)

const (
	BookingStatusPending   = "PENDING"
	BookingStatusPaid      = "PAID"
	BookingStatusExpired   = "EXPIRED"
	BookingStatusCancelled = "CANCELLED"
	BookingStatusRefunded  = "REFUNDED"

	SeatStatusLocked   = "LOCKED"
	SeatStatusBooked   = "BOOKED"
	SeatStatusReleased = "RELEASED"
	SeatStatusRefunded = "REFUNDED"
)

func IsValidScheduleStatus(status string) bool {
	switch status {
	case ScheduleStatusScheduled, ScheduleStatusOngoing, ScheduleStatusCompleted, ScheduleStatusCancelled:
		return true
	default:
		return false
	}
}

type User struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	FullName     string    `gorm:"size:100;not null" json:"full_name"`
	Email        string    `gorm:"size:100;uniqueIndex;not null" json:"email"`
	PasswordHash string    `gorm:"size:255;not null" json:"-"`
	Phone        *string   `gorm:"size:20" json:"phone,omitempty"`
	Role         string    `gorm:"size:20;not null;default:CUSTOMER" json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type City struct {
	ID       int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	Name     string `gorm:"size:100;uniqueIndex;not null" json:"name"`
	Province string `gorm:"size:100;not null" json:"province"`
}

type Cinema struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	CityID    int64     `gorm:"not null" json:"city_id"`
	Name      string    `gorm:"size:150;not null" json:"name"`
	Address   string    `gorm:"type:text;not null" json:"address"`
	CreatedAt time.Time `json:"created_at"`
	City      *City     `gorm:"foreignKey:CityID" json:"city,omitempty"`
}

type Studio struct {
	ID         int64   `gorm:"primaryKey;autoIncrement" json:"id"`
	CinemaID   int64   `gorm:"not null" json:"cinema_id"`
	Name       string  `gorm:"size:50;not null" json:"name"`
	StudioType string  `gorm:"size:30;not null;default:REGULAR" json:"studio_type"`
	TotalSeats int     `gorm:"not null" json:"total_seats"`
	Cinema     *Cinema `gorm:"foreignKey:CinemaID" json:"cinema,omitempty"`
}

type Seat struct {
	ID         int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	StudioID   int64  `gorm:"not null" json:"studio_id"`
	RowLabel   string `gorm:"size:5;not null" json:"row_label"`
	SeatNumber int    `gorm:"not null" json:"seat_number"`
	SeatType   string `gorm:"size:20;not null;default:REGULAR" json:"seat_type"`
}

type Movie struct {
	ID              int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Title           string     `gorm:"size:200;not null" json:"title"`
	Genre           *string    `gorm:"size:100" json:"genre,omitempty"`
	DurationMinutes int        `gorm:"not null" json:"duration_minutes"`
	Rating          *string    `gorm:"size:10" json:"rating,omitempty"`
	Synopsis        *string    `gorm:"type:text" json:"synopsis,omitempty"`
	ReleaseDate     *time.Time `gorm:"type:date" json:"release_date,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type Schedule struct {
	ID                 int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	MovieID            int64     `gorm:"not null" json:"movie_id"`
	StudioID           int64     `gorm:"not null" json:"studio_id"`
	StartTime          time.Time `gorm:"not null" json:"start_time"`
	EndTime            time.Time `gorm:"not null" json:"end_time"`
	Price              float64   `gorm:"type:numeric(12,2);not null" json:"price"`
	Status             string    `gorm:"size:20;not null;default:SCHEDULED" json:"status"`
	CancellationReason *string   `gorm:"type:text" json:"cancellation_reason,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	Movie              *Movie    `gorm:"foreignKey:MovieID" json:"movie,omitempty"`
	Studio             *Studio   `gorm:"foreignKey:StudioID" json:"studio,omitempty"`
}

type Booking struct {
	ID          int64         `gorm:"primaryKey;autoIncrement" json:"id"`
	BookingCode string        `gorm:"size:30;uniqueIndex;not null" json:"booking_code"`
	UserID      int64         `gorm:"not null" json:"user_id"`
	ScheduleID  int64         `gorm:"not null" json:"schedule_id"`
	TotalAmount float64       `gorm:"type:numeric(12,2);not null" json:"total_amount"`
	Status      string        `gorm:"size:20;not null;default:PENDING" json:"status"`
	ExpiredAt   time.Time     `gorm:"not null" json:"expired_at"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	Schedule    *Schedule     `gorm:"foreignKey:ScheduleID" json:"schedule,omitempty"`
	Seats       []BookingSeat `gorm:"foreignKey:BookingID" json:"seats,omitempty"`
	Payment     *Payment      `gorm:"foreignKey:BookingID" json:"payment,omitempty"`
	Refund      *Refund       `gorm:"foreignKey:BookingID" json:"refund,omitempty"`
}

type BookingSeat struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	BookingID  int64     `gorm:"not null" json:"booking_id"`
	ScheduleID int64     `gorm:"not null" json:"schedule_id"`
	SeatID     int64     `gorm:"not null" json:"seat_id"`
	Price      float64   `gorm:"type:numeric(12,2);not null" json:"price"`
	Status     string    `gorm:"size:20;not null;default:LOCKED" json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	Seat       *Seat     `gorm:"foreignKey:SeatID" json:"seat,omitempty"`
}

type Payment struct {
	ID               int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	BookingID        int64      `gorm:"uniqueIndex;not null" json:"booking_id"`
	PaymentMethod    string     `gorm:"size:50;not null" json:"payment_method"`
	GatewayReference *string    `gorm:"size:100;uniqueIndex" json:"gateway_reference,omitempty"`
	Amount           float64    `gorm:"type:numeric(12,2);not null" json:"amount"`
	Status           string     `gorm:"size:20;not null;default:PENDING" json:"status"`
	PaidAt           *time.Time `json:"paid_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

type Refund struct {
	ID             int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	BookingID      int64      `gorm:"uniqueIndex;not null" json:"booking_id"`
	PaymentID      int64      `gorm:"not null" json:"payment_id"`
	IdempotencyKey string     `gorm:"size:100;uniqueIndex;not null" json:"idempotency_key"`
	RefundAmount   float64    `gorm:"type:numeric(12,2);not null" json:"refund_amount"`
	Reason         string     `gorm:"type:text;not null" json:"reason"`
	InitiatedBy    string     `gorm:"size:20;not null;default:CINEMA" json:"initiated_by"`
	Status         string     `gorm:"size:20;not null;default:PENDING" json:"status"`
	RefundedAt     *time.Time `json:"refunded_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type JWTClaims struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type RegisterRequest struct {
	FullName string  `json:"full_name" binding:"required,max=100"`
	Email    string  `json:"email" binding:"required,email,max=100"`
	Password string  `json:"password" binding:"required,min=6,max=72"`
	Phone    *string `json:"phone" binding:"omitempty,max=20"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token     string    `json:"token"`
	TokenType string    `json:"token_type"`
	ExpiresAt time.Time `json:"expires_at"`
	User      *User     `json:"user"`
}

type CreateScheduleRequest struct {
	MovieID   int64      `json:"movie_id" binding:"required,gt=0"`
	StudioID  int64      `json:"studio_id" binding:"required,gt=0"`
	StartTime time.Time  `json:"start_time" binding:"required"`
	EndTime   *time.Time `json:"end_time"`
	Price     *float64   `json:"price" binding:"required,gte=0"`
}

type UpdateScheduleRequest struct {
	MovieID            *int64     `json:"movie_id" binding:"omitempty,gt=0"`
	StudioID           *int64     `json:"studio_id" binding:"omitempty,gt=0"`
	StartTime          *time.Time `json:"start_time"`
	EndTime            *time.Time `json:"end_time"`
	Price              *float64   `json:"price" binding:"omitempty,gte=0"`
	Status             *string    `json:"status"`
	CancellationReason *string    `json:"cancellation_reason"`
}

type CancelScheduleRequest struct {
	CancellationReason string `json:"cancellation_reason" binding:"required"`
}

type ScheduleFilter struct {
	MovieID  *int64
	StudioID *int64
	CinemaID *int64
	Status   string
	Date     string
	Page     int
	Limit    int
}

type ScheduleListResponse struct {
	Items []Schedule `json:"items"`
	Total int64      `json:"total"`
	Page  int        `json:"page"`
	Limit int        `json:"limit"`
}

type SeatAvailability struct {
	ID         int64  `json:"id"`
	StudioID   int64  `json:"studio_id"`
	RowLabel   string `json:"row_label"`
	SeatNumber int    `json:"seat_number"`
	SeatType   string `json:"seat_type"`
	Status     string `json:"status"`
}

type CreateBookingRequest struct {
	ScheduleID int64   `json:"schedule_id" binding:"required,gt=0"`
	SeatIDs    []int64 `json:"seat_ids" binding:"required,min=1,max=8"`
}

type PayBookingRequest struct {
	PaymentMethod string `json:"payment_method" binding:"required"`
	PromoCode     string `json:"promo_code"`
}


