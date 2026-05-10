package services

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/sidz111/pgbook/internals/models"
	"github.com/sidz111/pgbook/internals/repositories"
)

type RoomService interface {
	// CRUD Operations
	CreateRoom(ctx context.Context, room *models.Room) error
	GetRoomByID(ctx context.Context, id uuid.UUID) (*models.Room, error)
	GetRoomsByPG(ctx context.Context, pgID uuid.UUID) ([]models.Room, error)
	UpdateRoom(ctx context.Context, room *models.Room) error
	DeleteRoom(ctx context.Context, id uuid.UUID) error

	// Occupancy Management
	GetAvailableRooms(ctx context.Context, pgID uuid.UUID) ([]models.Room, error)
	GetRoomOccupancyDetails(ctx context.Context, pgID uuid.UUID) (map[string]interface{}, error)
	CheckRoomAvailability(ctx context.Context, roomID uuid.UUID) (bool, error)
	GetRoomCapacityStatus(ctx context.Context, roomID uuid.UUID) (map[string]interface{}, error)

	// Analytics
	GetOccupancyRate(ctx context.Context, pgID uuid.UUID) (float64, error)
	GetVacantRoomCount(ctx context.Context, pgID uuid.UUID) (int64, error)
}

type roomService struct {
	roomRepo repositories.RoomRepository
	pgRepo   repositories.PGRepository
	logger   *slog.Logger
}

func NewRoomService(
	roomRepo repositories.RoomRepository,
	pgRepo repositories.PGRepository,
) RoomService {
	return &roomService{
		roomRepo: roomRepo,
		pgRepo:   pgRepo,
		logger:   slog.Default(),
	}
}

// CreateRoom creates a new room with validation
func (s *roomService) CreateRoom(ctx context.Context, room *models.Room) error {
	// Validation
	if room.PGID == uuid.Nil {
		return errors.New("PG ID is required")
	}
	if room.RoomNumber == "" {
		return errors.New("room number is required")
	}
	if room.Capacity <= 0 {
		return errors.New("room capacity must be greater than 0")
	}
	if room.RentAmount <= 0 {
		return errors.New("rent amount must be greater than 0")
	}
	if room.SharingType == "" {
		return errors.New("sharing type is required")
	}

	// Verify PG exists
	_, err := s.pgRepo.GetPGByID(ctx, room.PGID)
	if err != nil {
		return errors.New("PG not found")
	}

	room.ID = uuid.New()
	room.Occupied = 0

	if err := s.roomRepo.CreateRoom(ctx, room); err != nil {
		s.logger.Error("Failed to create room", "error", err, "pg_id", room.PGID)
		return errors.New("failed to create room")
	}

	s.logger.Info("Room created successfully", "room_id", room.ID, "pg_id", room.PGID)
	return nil
}

// GetRoomByID retrieves room details
func (s *roomService) GetRoomByID(ctx context.Context, id uuid.UUID) (*models.Room, error) {
	if id == uuid.Nil {
		return nil, errors.New("invalid room ID")
	}

	room, err := s.roomRepo.GetRoomByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get room", "error", err, "room_id", id)
		return nil, errors.New("room not found")
	}

	return room, nil
}

// GetRoomsByPG retrieves all rooms for a PG
func (s *roomService) GetRoomsByPG(ctx context.Context, pgID uuid.UUID) ([]models.Room, error) {
	if pgID == uuid.Nil {
		return nil, errors.New("invalid PG ID")
	}

	rooms, err := s.roomRepo.GetRoomsByPGID(ctx, pgID)
	if err != nil {
		s.logger.Error("Failed to get rooms", "error", err, "pg_id", pgID)
		return nil, errors.New("failed to fetch rooms")
	}

	return rooms, nil
}

// UpdateRoom updates room details
func (s *roomService) UpdateRoom(ctx context.Context, room *models.Room) error {
	if room.ID == uuid.Nil {
		return errors.New("room ID is required")
	}

	// Retrieve existing room
	existingRoom, err := s.roomRepo.GetRoomByID(ctx, room.ID)
	if err != nil {
		return errors.New("room not found")
	}

	// Preserve critical fields
	room.PGID = existingRoom.PGID
	room.Occupied = existingRoom.Occupied
	room.CreatedAt = existingRoom.CreatedAt

	if err := s.roomRepo.UpdateRoom(ctx, room); err != nil {
		s.logger.Error("Failed to update room", "error", err, "room_id", room.ID)
		return errors.New("failed to update room")
	}

	s.logger.Info("Room updated successfully", "room_id", room.ID)
	return nil
}

// DeleteRoom deletes a room
func (s *roomService) DeleteRoom(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return errors.New("invalid room ID")
	}

	// Check if room has tenants
	room, err := s.roomRepo.GetRoomByID(ctx, id)
	if err != nil {
		return errors.New("room not found")
	}

	if room.Occupied > 0 {
		return errors.New("cannot delete room with active tenants")
	}

	if err := s.roomRepo.DeleteRoom(ctx, id); err != nil {
		s.logger.Error("Failed to delete room", "error", err, "room_id", id)
		return errors.New("failed to delete room")
	}

	s.logger.Info("Room deleted successfully", "room_id", id)
	return nil
}

// GetAvailableRooms retrieves all rooms with available capacity
func (s *roomService) GetAvailableRooms(ctx context.Context, pgID uuid.UUID) ([]models.Room, error) {
	if pgID == uuid.Nil {
		return nil, errors.New("invalid PG ID")
	}

	rooms, err := s.roomRepo.GetRoomsByPGID(ctx, pgID)
	if err != nil {
		s.logger.Error("Failed to get rooms", "error", err, "pg_id", pgID)
		return nil, errors.New("failed to fetch rooms")
	}

	// Filter rooms with available capacity
	var availableRooms []models.Room
	for _, room := range rooms {
		if room.Occupied < room.Capacity {
			availableRooms = append(availableRooms, room)
		}
	}

	return availableRooms, nil
}

// GetRoomOccupancyDetails provides detailed occupancy information
func (s *roomService) GetRoomOccupancyDetails(ctx context.Context, pgID uuid.UUID) (map[string]interface{}, error) {
	if pgID == uuid.Nil {
		return nil, errors.New("invalid PG ID")
	}

	rooms, err := s.roomRepo.GetRoomsByPGID(ctx, pgID)
	if err != nil {
		return nil, errors.New("failed to fetch rooms")
	}

	totalCapacity := int8(0)
	totalOccupied := int8(0)
	totalVacant := int8(0)
	occupancyDetails := make([]map[string]interface{}, 0)

	for _, room := range rooms {
		totalCapacity += room.Capacity
		totalOccupied += room.Occupied
		vacant := room.Capacity - room.Occupied
		totalVacant += vacant

		occupancyDetails = append(occupancyDetails, map[string]interface{}{
			"room_id":     room.ID,
			"room_number": room.RoomNumber,
			"capacity":    room.Capacity,
			"occupied":    room.Occupied,
			"vacant":      vacant,
		})
	}

	occupancyRate := 0.0
	if totalCapacity > 0 {
		occupancyRate = float64(totalOccupied) / float64(totalCapacity) * 100
	}

	return map[string]interface{}{
		"total_rooms":    len(rooms),
		"total_capacity": totalCapacity,
		"total_occupied": totalOccupied,
		"total_vacant":   totalVacant,
		"occupancy_rate": occupancyRate,
		"details":        occupancyDetails,
	}, nil
}

// CheckRoomAvailability checks if a room has available capacity
func (s *roomService) CheckRoomAvailability(ctx context.Context, roomID uuid.UUID) (bool, error) {
	if roomID == uuid.Nil {
		return false, errors.New("invalid room ID")
	}

	room, err := s.roomRepo.GetRoomByID(ctx, roomID)
	if err != nil {
		return false, errors.New("room not found")
	}

	return room.Occupied < room.Capacity, nil
}

// GetRoomCapacityStatus provides capacity status for a room
func (s *roomService) GetRoomCapacityStatus(ctx context.Context, roomID uuid.UUID) (map[string]interface{}, error) {
	if roomID == uuid.Nil {
		return nil, errors.New("invalid room ID")
	}

	room, err := s.roomRepo.GetRoomByID(ctx, roomID)
	if err != nil {
		return nil, errors.New("room not found")
	}

	vacant := room.Capacity - room.Occupied
	occupancyRate := 0.0
	if room.Capacity > 0 {
		occupancyRate = float64(room.Occupied) / float64(room.Capacity) * 100
	}

	return map[string]interface{}{
		"room_id":        room.ID,
		"room_number":    room.RoomNumber,
		"capacity":       room.Capacity,
		"occupied":       room.Occupied,
		"vacant":         vacant,
		"occupancy_rate": occupancyRate,
		"is_full":        room.Occupied >= room.Capacity,
		"is_empty":       room.Occupied == 0,
	}, nil
}

// GetOccupancyRate calculates overall occupancy rate for a PG
func (s *roomService) GetOccupancyRate(ctx context.Context, pgID uuid.UUID) (float64, error) {
	if pgID == uuid.Nil {
		return 0, errors.New("invalid PG ID")
	}

	rooms, err := s.roomRepo.GetRoomsByPGID(ctx, pgID)
	if err != nil {
		return 0, errors.New("failed to fetch rooms")
	}

	if len(rooms) == 0 {
		return 0, nil
	}

	totalCapacity := int8(0)
	totalOccupied := int8(0)

	for _, room := range rooms {
		totalCapacity += room.Capacity
		totalOccupied += room.Occupied
	}

	if totalCapacity == 0 {
		return 0, nil
	}

	return float64(totalOccupied) / float64(totalCapacity) * 100, nil
}

// GetVacantRoomCount returns count of completely vacant rooms
func (s *roomService) GetVacantRoomCount(ctx context.Context, pgID uuid.UUID) (int64, error) {
	if pgID == uuid.Nil {
		return 0, errors.New("invalid PG ID")
	}

	rooms, err := s.roomRepo.GetRoomsByPGID(ctx, pgID)
	if err != nil {
		return 0, errors.New("failed to fetch rooms")
	}

	vacantCount := int64(0)
	for _, room := range rooms {
		if room.Occupied == 0 {
			vacantCount++
		}
	}

	return vacantCount, nil
}
