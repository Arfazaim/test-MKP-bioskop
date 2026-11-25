package handler

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"bioskop-mkp/internal/middleware"
	"bioskop-mkp/internal/model"
	"bioskop-mkp/internal/repository"
	"bioskop-mkp/internal/service"

	"github.com/gin-gonic/gin"
)

type BookingHandler struct {
	bookingService service.BookingService
}

func NewBookingHandler(bookingService service.BookingService) *BookingHandler {
	return &BookingHandler{bookingService: bookingService}
}

func (h *BookingHandler) GetCatalogMetadata(c *gin.Context) {
	meta, err := h.bookingService.GetCatalogMetadata(c.Request.Context())
	if err != nil {
		log.Printf("catalog metadata error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "terjadi kesalahan pada server"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "data katalog berhasil diambil",
		"data":    meta,
	})
}

func (h *BookingHandler) GetScheduleSeats(c *gin.Context) {
	scheduleID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || scheduleID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id jadwal tidak valid"})
		return
	}

	resp, err := h.bookingService.GetScheduleSeats(c.Request.Context(), scheduleID)
	if err != nil {
		respondBookingError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "denah kursi berhasil diambil",
		"data":    resp,
	})
}

func (h *BookingHandler) Create(c *gin.Context) {
	userID := c.GetInt64(middleware.ContextUserIDKey)
	var req model.CreateBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pilih minimal 1 dan maksimal 8 kursi"})
		return
	}

	booking, err := h.bookingService.CreateBooking(c.Request.Context(), userID, req)
	if err != nil {
		respondBookingError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"message": "kursi berhasil ditahan selama 10 menit, silakan selesaikan pembayaran",
		"data":    booking,
	})
}

func (h *BookingHandler) ListMyBookings(c *gin.Context) {
	userID := c.GetInt64(middleware.ContextUserIDKey)
	bookings, err := h.bookingService.ListMyBookings(c.Request.Context(), userID)
	if err != nil {
		respondBookingError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "daftar pesanan berhasil diambil",
		"data":    bookings,
	})
}

func (h *BookingHandler) GetByID(c *gin.Context) {
	bookingID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || bookingID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id pesanan tidak valid"})
		return
	}

	userID := c.GetInt64(middleware.ContextUserIDKey)
	role := c.GetString(middleware.ContextRoleKey)
	booking, err := h.bookingService.GetBooking(c.Request.Context(), bookingID, userID, role)
	if err != nil {
		respondBookingError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "detail pesanan berhasil diambil",
		"data":    booking,
	})
}

func (h *BookingHandler) Pay(c *gin.Context) {
	bookingID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || bookingID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id pesanan tidak valid"})
		return
	}

	var req model.PayBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pilih metode pembayaran terlebih dahulu"})
		return
	}

	userID := c.GetInt64(middleware.ContextUserIDKey)
	booking, err := h.bookingService.PayBooking(c.Request.Context(), bookingID, userID, req)
	if err != nil {
		respondBookingError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "pembayaran berhasil, tiket anda sudah siap",
		"data":    booking,
	})
}

func (h *BookingHandler) Cancel(c *gin.Context) {
	bookingID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || bookingID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id pesanan tidak valid"})
		return
	}

	userID := c.GetInt64(middleware.ContextUserIDKey)
	booking, err := h.bookingService.CancelBooking(c.Request.Context(), bookingID, userID)
	if err != nil {
		respondBookingError(c, err)
		return
	}
	msg := "pesanan dibatalkan dan kursi sudah dibuka kembali"
	if booking.Status == model.BookingStatusRefunded {
		msg = "tiket lunas dibatalkan, dana dikembalikan dan kursi otomatis tersedia kembali"
	}
	c.JSON(http.StatusOK, gin.H{
		"message": msg,
		"data":    booking,
	})
}

func respondBookingError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidInput),
		errors.Is(err, service.ErrInvalidPaymentMethod),
		errors.Is(err, repository.ErrInvalidSeats):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrForbiddenBooking):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, repository.ErrBookingNotFound),
		errors.Is(err, repository.ErrScheduleNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, repository.ErrSeatAlreadyTaken),
		errors.Is(err, repository.ErrScheduleNotAvailable),
		errors.Is(err, repository.ErrBookingExpired),
		errors.Is(err, repository.ErrBookingNotPending),
		errors.Is(err, repository.ErrShowAlreadyStarted):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		log.Printf("booking handler error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "terjadi kesalahan pada server"})
	}
}

