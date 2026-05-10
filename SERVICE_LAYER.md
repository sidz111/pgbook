# PGBook Service Layer Documentation

This document provides a comprehensive overview of all services in the PGBook project.

## Architecture Overview

The service layer implements business logic on top of repositories. All services follow the interface-based pattern and use dependency injection for repositories.

```
Handlers (HTTP Layer)
    ↓
Services (Business Logic Layer)
    ↓
Repositories (Data Access Layer)
    ↓
Database (PostgreSQL)
```

---

## Service Interfaces & Methods

### 1. **AuthService** - Authentication & Authorization

**File:** `internals/services/auth_service.go`

#### Methods:

- `Register(ctx context.Context, user *models.User) error`
  - Registers a new user with email validation
  - Password is hashed using bcrypt
  - Validates email uniqueness

- `Login(ctx context.Context, email, password string) (accessToken, refreshToken string, user *models.User, error)`
  - Authenticates user with email/password
  - Returns JWT access token (15 min expiry) and refresh token (7 days expiry)
  - Stores refresh token in database for validation

- `RefreshToken(ctx context.Context, refreshToken string) (newAccessToken string, error)`
  - Generates new access token from valid refresh token
  - Validates refresh token against stored token

- `Logout(ctx context.Context, userID string) error`
  - Clears refresh token from database
  - Invalidates current session

---

### 2. **PGService** - PG (Paying Guest) Management

**File:** `internals/services/pg_service.go`

#### Methods - CRUD:

- `CreatePG(ctx context.Context, pg *models.PG) error`
  - Creates new PG with validation
  - Ensures owner doesn't have multiple PGs
  - Auto-generates UUID

- `GetPGByID(ctx context.Context, id uuid.UUID) (*models.PG, error)`
  - Retrieves PG with all relationships (rooms, tenants, subscriptions)

- `GetPGByOwner(ctx context.Context, ownerID uuid.UUID) (*models.PG, error)`
  - Fetches PG for specific owner

- `UpdatePG(ctx context.Context, pg *models.PG) error`
  - Updates PG details (prevents critical field changes)

- `DeletePG(ctx context.Context, id uuid.UUID) error`
  - Deletes PG (soft delete)
  - Validates no active tenants exist

#### Methods - Analytics:

- `GetAllPGs(ctx context.Context, limit, offset int) ([]models.PG, error)`
  - Paginated retrieval of all PGs

- `GetPGStatistics(ctx context.Context, pgID uuid.UUID) (map[string]interface{}, error)`
  - Returns total rooms and active tenants count

- `GetDashboardData(ctx context.Context, pgID uuid.UUID) (map[string]interface{}, error)`
  - Comprehensive dashboard data for owner
  - Includes: subscription status, room availability, tenant count

#### Methods - Subscription:

- `ActivateTrial(ctx context.Context, pgID uuid.UUID) error`
  - Activates 30-day trial for new PG

- `CheckSubscriptionStatus(ctx context.Context, pgID uuid.UUID) (bool, error)`
  - Checks if PG has active subscription

---

### 3. **RoomService** - Room Management

**File:** `internals/services/room_service.go`

#### Methods - CRUD:

- `CreateRoom(ctx context.Context, room *models.Room) error`
  - Creates room with validation (capacity > 0, rent > 0)
  - Verifies PG exists

- `GetRoomByID(ctx context.Context, id uuid.UUID) (*models.Room, error)`
  - Retrieves room with tenants

- `GetRoomsByPG(ctx context.Context, pgID uuid.UUID) ([]models.Room, error)`
  - All rooms for a specific PG

- `UpdateRoom(ctx context.Context, room *models.Room) error`
  - Updates room details (preserves PGID and occupancy)

- `DeleteRoom(ctx context.Context, id uuid.UUID) error`
  - Deletes room (validates no active tenants)

#### Methods - Occupancy:

- `GetAvailableRooms(ctx context.Context, pgID uuid.UUID) ([]models.Room, error)`
  - Returns rooms with available capacity

- `GetRoomOccupancyDetails(ctx context.Context, pgID uuid.UUID) (map[string]interface{}, error)`
  - Detailed occupancy breakdown per room
  - Calculates occupancy rate percentage

- `CheckRoomAvailability(ctx context.Context, roomID uuid.UUID) (bool, error)`
  - Boolean check if room has space

- `GetRoomCapacityStatus(ctx context.Context, roomID uuid.UUID) (map[string]interface{}, error)`
  - Capacity, occupied, vacant, occupancy rate

#### Methods - Analytics:

- `GetOccupancyRate(ctx context.Context, pgID uuid.UUID) (float64, error)`
  - Overall occupancy percentage for PG

- `GetVacantRoomCount(ctx context.Context, pgID uuid.UUID) (int64, error)`
  - Count of completely empty rooms

---

### 4. **TenantService** - Tenant Lifecycle Management

**File:** `internals/services/tenant_service.go`

#### Methods - CRUD:

- `CreateTenant(ctx context.Context, tenant *models.Tenant) error`
  - Creates tenant with room capacity validation
  - Auto-increments room occupancy
  - Sets joining_date to now if not provided

- `GetTenantByID(ctx context.Context, id uuid.UUID) (*models.Tenant, error)`
  - Retrieves tenant with payment history

- `GetTenantByUserID(ctx context.Context, userID uuid.UUID) (*models.Tenant, error)`
  - Fetch tenant for a user

- `GetTenantsByPG(ctx context.Context, pgID uuid.UUID) ([]models.Tenant, error)`
  - All active tenants in PG

- `UpdateTenant(ctx context.Context, tenant *models.Tenant) error`
  - Updates tenant info (preserves user/PG/room assignments)

- `UpdateProfilePhoto(ctx context.Context, tenantID uuid.UUID, photoURL string) error`
  - Updates profile picture

#### Methods - Notice Period:

- `InitiateNotice(ctx context.Context, tenantID uuid.UUID, noticePeriodDays int) error`
  - Starts exit notice period
  - Calculates exit_date automatically
  - Validates notice period ≤ 180 days

- `CancelNotice(ctx context.Context, tenantID uuid.UUID) error`
  - Cancels ongoing notice period

- `GetRemainingNoticeDays(ctx context.Context, tenantID uuid.UUID) (int, error)`
  - Days remaining until exit date

- `OffboardTenant(ctx context.Context, tenantID uuid.UUID) error`
  - Deactivates tenant, decrements room occupancy
  - Handles automatic exit

- `GetTenantsOnNotice(ctx context.Context, pgID uuid.UUID) ([]models.Tenant, error)`
  - All tenants with active notice in PG

- `ProcessExpiredNotices(ctx context.Context) error`
  - Cron job to auto-offboard tenants whose notice expired

#### Methods - Analytics:

- `GetTenantStatus(ctx context.Context, tenantID uuid.UUID) (map[string]interface{}, error)`
  - Full status including notice period info

- `GetPGTenantStats(ctx context.Context, pgID uuid.UUID) (map[string]interface{}, error)`
  - Total active, on notice, upcoming expiries

- `GetTenantHistory(ctx context.Context, pgID uuid.UUID) ([]models.Tenant, error)`
  - Historical tenant records

---

### 5. **SubscriptionService** - Subscription Management

**File:** `internals/services/subscription_service.go`

#### Methods - CRUD:

- `CreateSubscription(ctx context.Context, subscription *models.Subscription) error`
  - Creates subscription request with payment proof
  - Default status: "pending"
  - Validates amount > 0 and proof_url

- `GetSubscriptionByID(ctx context.Context, id uuid.UUID) (*models.Subscription, error)`
  - Retrieves subscription details

- `GetSubscriptionsByPG(ctx context.Context, pgID uuid.UUID) ([]models.Subscription, error)`
  - All subscriptions for a PG

#### Methods - Admin Operations:

- `ApproveSubscription(ctx context.Context, subID uuid.UUID, months int, adminName string) error`
  - Approves pending subscription
  - Sets expiry_date and marks PG as subscribed
  - Validates 1-36 months duration

- `RejectSubscription(ctx context.Context, subID uuid.UUID) error`
  - Rejects payment (if screenshot invalid)

#### Methods - Subscription Management:

- `GetActiveSubscription(ctx context.Context, pgID uuid.UUID) (*models.Subscription, error)`
  - Returns current active subscription

- `IsSubscriptionActive(ctx context.Context, pgID uuid.UUID) (bool, error)`
  - Boolean check for active subscription

- `GetSubscriptionStatus(ctx context.Context, subID uuid.UUID) (string, error)`
  - Returns status (pending/active/rejected/expired)

- `RenewSubscription(ctx context.Context, pgID uuid.UUID, months int, adminName string) error`
  - Extends subscription from current expiry

#### Methods - Cron/Utilities:

- `GetPendingSubscriptions(ctx context.Context) ([]models.Subscription, error)`
  - All subscriptions awaiting admin approval

- `GetExpiredSubscriptions(ctx context.Context) ([]models.Subscription, error)`
  - All expired subscriptions

- `ProcessExpiredSubscriptions(ctx context.Context) error`
  - Cron job to mark expired subs and update PG status

- `GetSubscriptionExpiringSoon(ctx context.Context, daysThreshold int) ([]models.Subscription, error)`
  - Subscriptions expiring within N days (default 7)

#### Methods - Analytics:

- `GetSubscriptionStats(ctx context.Context) (map[string]interface{}, error)`
  - Total pending/active/expired and approval rate

---

### 6. **PaymentService** - Payment & Rent Management

**File:** `internals/services/payment_service.go`

#### Methods - CRUD:

- `CreatePayment(ctx context.Context, payment *models.Payment) error`
  - Creates payment record
  - Default status: "pending"
  - Validates tenant, PG exist, amount > 0

- `GetPaymentByID(ctx context.Context, id uuid.UUID) (*models.Payment, error)`
  - Retrieves payment

- `GetPaymentsByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.Payment, error)`
  - All payments for tenant

- `GetPaymentsByPG(ctx context.Context, pgID uuid.UUID) ([]models.Payment, error)`
  - All payments in PG

#### Methods - Payment Management:

- `UpdatePaymentStatus(ctx context.Context, paymentID uuid.UUID, status string) error`
  - Updates status (pending/verified/rejected)

- `VerifyPayment(ctx context.Context, paymentID uuid.UUID, adminRemarks string) error`
  - Marks payment as verified with remarks

- `RejectPayment(ctx context.Context, paymentID uuid.UUID, reason string) error`
  - Marks payment as rejected with reason

#### Methods - Analytics:

- `GetPendingPayments(ctx context.Context, pgID uuid.UUID) ([]models.Payment, error)`
  - All pending/unverified payments

- `GetPaymentStats(ctx context.Context, pgID uuid.UUID) (map[string]interface{}, error)`
  - Total collected/pending/rejected, collection rate

- `GetTenantPaymentHistory(ctx context.Context, tenantID uuid.UUID, months int) ([]models.Payment, error)`
  - Last N months payment records (default 6)

- `GetMonthlyCollectionStats(ctx context.Context, pgID uuid.UUID) (map[string]interface{}, error)`
  - Monthly breakdown: collected, pending, rejected

- `GetOutstandingPayments(ctx context.Context, pgID uuid.UUID) ([]models.Payment, error)`
  - All pending and rejected payments

#### Methods - Monthly Rent:

- `GetMonthlyOutstanding(ctx context.Context, tenantID uuid.UUID, month string) (*models.Payment, error)`
  - Checks outstanding payment for specific month

- `InitiateMonthlyPayment(ctx context.Context, tenantID uuid.UUID, month string, amount float64) error`
  - Creates monthly rent entry
  - Validates no duplicate for same month

---

## Key Design Patterns

### 1. **Dependency Injection**

All services receive required repositories via constructor:

```go
func NewPGService(pgRepo, roomRepo, tenantRepo, subscriptionRepo) PGService
```

### 2. **Interface-Based Design**

Each service is defined as an interface for testability and flexibility:

```go
type PGService interface {
    CreatePG(ctx context.Context, pg *models.PG) error
    // ... methods
}
```

### 3. **Context Usage**

All methods accept `context.Context` for:

- Request timeout
- Cancellation
- Request-scoped values

### 4. **Error Handling**

- Explicit error messages
- Validation at service layer
- Logging of failures

### 5. **Structured Logging**

Uses `log/slog` for JSON-formatted logs:

```go
s.logger.Info("Payment verified", "payment_id", paymentID, "tenant_id", tenantID)
s.logger.Error("Failed to create payment", "error", err, "tenant_id", tenantID)
```

### 6. **Transaction Safety**

- Repository methods handle transactions
- Service layer ensures consistency

### 7. **Business Logic Separation**

- Notice period calculations (TenantService)
- Occupancy management (RoomService)
- Payment verification workflows (PaymentService)
- Subscription approval workflows (SubscriptionService)

---

## Usage Example in Handlers

```go
// In handler initialization (main.go)
authService := services.NewAuthService(userRepo, jwtSecret)
pgService := services.NewPGService(pgRepo, roomRepo, tenantRepo, subscriptionRepo)
tenantService := services.NewTenantService(tenantRepo, roomRepo, pgRepo)
paymentService := services.NewPaymentService(paymentRepo, tenantRepo, pgRepo, roomRepo)

// In handler method
func (h *PGHandler) CreatePG(c *gin.Context) {
    var req CreatePGRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    pg := &models.PG{
        UserID: userIDFromContext,
        Name: req.Name,
        // ...
    }

    if err := h.pgService.CreatePG(c.Request.Context(), pg); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusCreated, gin.H{"message": "PG created"})
}
```

---

## Service Initialization

All services should be initialized in `main.go`:

```go
func main() {
    // Initialize repositories
    userRepo := repositories.NewUserRepository(config.DB)
    pgRepo := repositories.NewPGRepository(config.DB)
    roomRepo := repositories.NewRoomRepository(config.DB)
    tenantRepo := repositories.NewTenantRepository(config.DB)
    paymentRepo := repositories.NewPaymentRepository(config.DB)
    subscriptionRepo := repositories.NewSubscriptionRepository(config.DB)

    // Initialize services
    authService := services.NewAuthService(userRepo, jwtSecret)
    pgService := services.NewPGService(pgRepo, roomRepo, tenantRepo, subscriptionRepo)
    roomService := services.NewRoomService(roomRepo, pgRepo)
    tenantService := services.NewTenantService(tenantRepo, roomRepo, pgRepo)
    paymentService := services.NewPaymentService(paymentRepo, tenantRepo, pgRepo, roomRepo)
    subscriptionService := services.NewSubscriptionService(subscriptionRepo, pgRepo)

    // Pass to handlers
    authHandler := handlers.NewAuthHandler(authService)
    pgHandler := handlers.NewPGHandler(pgService)
    // ... initialize other handlers
}
```

---

## Common Validation Rules

| Service             | Validation        | Error Message                                           |
| ------------------- | ----------------- | ------------------------------------------------------- |
| PGService           | UserID not nil    | "user_id is required"                                   |
| PGService           | Name not empty    | "PG name is required"                                   |
| RoomService         | Capacity > 0      | "room capacity must be greater than 0"                  |
| TenantService       | Room has capacity | "room is at full capacity"                              |
| TenantService       | Notice ≤ 180 days | "notice period cannot exceed 180 days"                  |
| PaymentService      | Amount > 0        | "payment amount must be greater than 0"                 |
| SubscriptionService | Months 1-36       | "subscription duration must be between 1 and 36 months" |

---

## Testing Recommendations

For each service, create corresponding `*_service_test.go` files:

```go
func TestCreatePG(t *testing.T) {
    // Mock repositories
    // Test validation
    // Test successful creation
    // Test duplicate prevention
}

func TestInitiateNotice(t *testing.T) {
    // Test notice period calculation
    // Test validation
    // Test inactive tenant rejection
}
```

Use `testify` for assertions and `go-sqlmock` for repository mocking.

---

## Cron Job Integration

Services support background job processing:

```go
// In main.go or scheduler
go func() {
    ticker := time.NewTicker(24 * time.Hour)
    for range ticker.C {
        tenantService.ProcessExpiredNotices(context.Background())
        subscriptionService.ProcessExpiredSubscriptions(context.Background())
    }
}()
```
