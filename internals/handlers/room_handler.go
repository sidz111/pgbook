package handlers

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sidz111/pgbook/internals/models"
	"github.com/sidz111/pgbook/internals/services"
)

type RoomHandler struct {
	roomService services.RoomService
	pgService   services.PGService
	logger      *slog.Logger
}

func NewRoomHandler(roomService services.RoomService, pgService services.PGService) *RoomHandler {
	return &RoomHandler{
		roomService: roomService,
		pgService:   pgService,
		logger:      slog.Default(),
	}
}

type CreateRoomRequest struct {
	RoomNumber  string  `json:"room_number" binding:"required"`
	SharingType string  `json:"sharing_type" binding:"required"`
	Capacity    int8    `json:"capacity" binding:"required,gt=0"`
	RentAmount  float64 `json:"rent_amount" binding:"required,gt=0"`
}

func (h *RoomHandler) CreateRoom(c *gin.Context) {
	pgID, err := uuid.Parse(c.Param("pg_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid PG ID"})
		return
	}

	if !verifyPGOwnerOrAdmin(c, h.pgService, pgID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	var req CreateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	room := &models.Room{
		PGID:        pgID,
		RoomNumber:  req.RoomNumber,
		SharingType: req.SharingType,
		Capacity:    req.Capacity,
		RentAmount:  req.RentAmount,
	}

	if err := h.roomService.CreateRoom(c.Request.Context(), room); err != nil {
		h.logger.Error("Failed to create room", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Room created successfully", "room_id": room.ID})
}

func (h *RoomHandler) GetRoomsByPG(c *gin.Context) {
	pgID, err := uuid.Parse(c.Param("pg_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid PG ID"})
		return
	}

	if !verifyPGOwnerOrAdmin(c, h.pgService, pgID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	rooms, err := h.roomService.GetRoomsByPG(c.Request.Context(), pgID)
	if err != nil {
		h.logger.Error("Failed to get rooms", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"rooms": rooms, "count": len(rooms)})
}

func (h *RoomHandler) GetRoomByID(c *gin.Context) {
	roomID, err := uuid.Parse(c.Param("room_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid room ID"})
		return
	}

	room, err := h.roomService.GetRoomByID(c.Request.Context(), roomID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "room not found"})
		return
	}

	if !verifyPGOwnerOrAdmin(c, h.pgService, room.PGID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	c.JSON(http.StatusOK, room)
}

func (h *RoomHandler) UpdateRoom(c *gin.Context) {
	roomID, err := uuid.Parse(c.Param("room_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid room ID"})
		return
	}

	room, err := h.roomService.GetRoomByID(c.Request.Context(), roomID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "room not found"})
		return
	}

	if !verifyPGOwnerOrAdmin(c, h.pgService, room.PGID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	var req CreateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	room.RoomNumber = req.RoomNumber
	room.SharingType = req.SharingType
	room.Capacity = req.Capacity
	room.RentAmount = req.RentAmount

	if err := h.roomService.UpdateRoom(c.Request.Context(), room); err != nil {
		h.logger.Error("Failed to update room", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Room updated successfully"})
}

func (h *RoomHandler) DeleteRoom(c *gin.Context) {
	roomID, err := uuid.Parse(c.Param("room_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid room ID"})
		return
	}

	room, err := h.roomService.GetRoomByID(c.Request.Context(), roomID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "room not found"})
		return
	}

	if !verifyPGOwnerOrAdmin(c, h.pgService, room.PGID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	if err := h.roomService.DeleteRoom(c.Request.Context(), roomID); err != nil {
		h.logger.Error("Failed to delete room", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Room deleted successfully"})
}

func (h *RoomHandler) GetRoomCapacityStatus(c *gin.Context) {
	roomID, err := uuid.Parse(c.Param("room_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid room ID"})
		return
	}

	room, err := h.roomService.GetRoomByID(c.Request.Context(), roomID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "room not found"})
		return
	}

	if !verifyPGOwnerOrAdmin(c, h.pgService, room.PGID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized access"})
		return
	}

	status, err := h.roomService.GetRoomCapacityStatus(c.Request.Context(), roomID)
	if err != nil {
		h.logger.Error("Failed to get room capacity status", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, status)
}
