package handler

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"bioskop-mkp/internal/model"
	"bioskop-mkp/internal/repository"
	"bioskop-mkp/internal/service"

	"github.com/gin-gonic/gin"
)

type ScheduleHandler struct {
	scheduleService service.ScheduleService
}

func NewScheduleHandler(scheduleService service.ScheduleService) *ScheduleHandler {
	return &ScheduleHandler{scheduleService: scheduleService}
}

func (h *ScheduleHandler) List(c *gin.Context) {
	var filter model.ScheduleFilter
	var err error

	if filter.MovieID, err = parseOptionalPositiveInt64(c.Query("movie_id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "parameter movie_id tidak valid"})
		return
	}
	if filter.StudioID, err = parseOptionalPositiveInt64(c.Query("studio_id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "parameter studio_id tidak valid"})
		return
	}
	if filter.CinemaID, err = parseOptionalPositiveInt64(c.Query("cinema_id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "parameter cinema_id tidak valid"})
		return
	}
	if raw := c.Query("page"); raw != "" {
		p, err := strconv.Atoi(raw)
		if err != nil || p <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "parameter page tidak valid"})
			return
		}
		filter.Page = p
	}
	if raw := c.Query("limit"); raw != "" {
		l, err := strconv.Atoi(raw)
		if err != nil || l <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "parameter limit tidak valid"})
			return
		}
		filter.Limit = l
	}

	filter.Status = c.Query("status")
	filter.Date = c.Query("date")

	resp, err := h.scheduleService.List(c.Request.Context(), filter)
	if err != nil {
		respondScheduleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "daftar jadwal tayang berhasil diambil",
		"data":    resp,
	})
}

func (h *ScheduleHandler) GetByID(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}

	schedule, err := h.scheduleService.GetByID(c.Request.Context(), id)
	if err != nil {
		respondScheduleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "detail jadwal tayang berhasil diambil",
		"data":    schedule,
	})
}

func (h *ScheduleHandler) Create(c *gin.Context) {
	var req model.CreateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data jadwal yang dikirim tidak lengkap atau formatnya salah"})
		return
	}

	schedule, err := h.scheduleService.Create(c.Request.Context(), req)
	if err != nil {
		respondScheduleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"message": "jadwal tayang berhasil ditambahkan",
		"data":    schedule,
	})
}

func (h *ScheduleHandler) Update(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}

	var req model.UpdateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data perubahan jadwal tidak sesuai format"})
		return
	}

	schedule, err := h.scheduleService.Update(c.Request.Context(), id, req)
	if err != nil {
		respondScheduleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "jadwal tayang berhasil diperbarui",
		"data":    schedule,
	})
}

func (h *ScheduleHandler) Cancel(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}

	var req model.CancelScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "alasan pembatalan wajib diisi"})
		return
	}

	schedule, err := h.scheduleService.Cancel(c.Request.Context(), id, req.CancellationReason)
	if err != nil {
		respondScheduleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "jadwal tayang berhasil dibatalkan",
		"data":    schedule,
	})
}

func (h *ScheduleHandler) Delete(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}

	if err := h.scheduleService.Delete(c.Request.Context(), id); err != nil {
		respondScheduleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "jadwal tayang berhasil dihapus"})
}

func parseIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id jadwal tidak valid"})
		return 0, false
	}
	return id, true
}

func parseOptionalPositiveInt64(raw string) (*int64, error) {
	if raw == "" {
		return nil, nil
	}
	val, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || val <= 0 {
		return nil, errors.New("harus berupa angka bulat positif")
	}
	return &val, nil
}

func respondScheduleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidInput),
		errors.Is(err, service.ErrInvalidTimeRange),
		errors.Is(err, service.ErrInvalidStatus),
		errors.Is(err, service.ErrCancellationReasonRequired),
		errors.Is(err, service.ErrInvalidDateFilter),
		errors.Is(err, service.ErrInvalidPrice):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, repository.ErrScheduleNotFound),
		errors.Is(err, repository.ErrMovieNotFound),
		errors.Is(err, repository.ErrStudioNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, repository.ErrScheduleOverlap),
		errors.Is(err, repository.ErrScheduleHasBookings),
		errors.Is(err, service.ErrInvalidScheduleState):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		log.Printf("schedule handler error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "terjadi kesalahan pada server"})
	}
}
