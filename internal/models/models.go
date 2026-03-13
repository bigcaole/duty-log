package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// JSONMap is used for JSON object persistence in PostgreSQL.
type JSONMap map[string]any

func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return "{}", nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (m *JSONMap) Scan(value any) error {
	if value == nil {
		*m = JSONMap{}
		return nil
	}
	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("unsupported JSONMap type: %T", value)
	}
	if len(raw) == 0 {
		*m = JSONMap{}
		return nil
	}
	return json.Unmarshal(raw, m)
}

// JSONSlice is used for JSON array persistence in PostgreSQL.
type JSONSlice []map[string]any

func (s JSONSlice) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (s *JSONSlice) Scan(value any) error {
	if value == nil {
		*s = JSONSlice{}
		return nil
	}
	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("unsupported JSONSlice type: %T", value)
	}
	if len(raw) == 0 {
		*s = JSONSlice{}
		return nil
	}
	return json.Unmarshal(raw, s)
}

type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"size:80;uniqueIndex;not null" json:"username"`
	PasswordHash string    `gorm:"size:256;not null" json:"-"`
	Email        string    `gorm:"size:120;uniqueIndex;not null" json:"email"`
	OTPSecret    string    `gorm:"size:64" json:"-"`
	IsActive     bool      `gorm:"default:true;not null" json:"is_active"`
	IsAdmin      bool      `gorm:"default:false;not null" json:"is_admin"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type IdcDutyRecord struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	UserID         *uint     `gorm:"index;uniqueIndex:idx_idc_duty_user_date" json:"user_id"`
	Date           time.Time `gorm:"type:date;not null;index;uniqueIndex:idx_idc_duty_user_date" json:"date"`
	DutyOps        string    `gorm:"size:100;not null" json:"duty_ops"`
	DutyIdc        string    `gorm:"size:100;not null" json:"duty_idc"`
	TaskCategoryID *uint     `json:"task_category_id"`
	Tasks          string    `gorm:"type:text" json:"tasks"`
	VisitsJSON     JSONSlice `gorm:"type:jsonb;not null;default:'[]'" json:"visits_json"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type WorkTicketType struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:100;uniqueIndex;not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type FaultType struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:100;uniqueIndex;not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type IDCOpsTicketType struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:100;uniqueIndex;not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type TaskCategory struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:100;uniqueIndex;not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Attachment struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Module      string    `gorm:"size:50;index;not null" json:"module"`
	ModuleID    uint      `gorm:"index;not null" json:"module_id"`
	Name        string    `gorm:"size:260;not null" json:"name"`
	ContentType string    `gorm:"size:120" json:"content_type"`
	Size        int64     `gorm:"not null;default:0" json:"size"`
	Data        []byte    `gorm:"type:bytea;not null" json:"-"`
	CreatedAt   time.Time `json:"created_at"`
}

type WorkTicket struct {
	ID                    uint      `gorm:"primaryKey" json:"id"`
	UserID                uint      `gorm:"not null;index" json:"user_id"`
	Date                  time.Time `gorm:"type:date;not null;default:CURRENT_DATE" json:"date"`
	DutyPerson            string    `gorm:"size:100;not null" json:"duty_person"`
	UserName              string    `gorm:"size:200;not null" json:"user_name"`
	TicketOrganization    string    `gorm:"size:200" json:"ticket_organization"`
	WorkTicketTypeID      uint      `gorm:"not null;index" json:"work_ticket_type_id"`
	OperationInfo         string    `gorm:"type:text;not null" json:"operation_info"`
	CustomerServicePerson string    `gorm:"size:100" json:"customer_service_person"`
	ProcessingStatus      string    `gorm:"size:20;default:'pending';not null" json:"processing_status"`
	Remarks               string    `gorm:"type:text" json:"remarks"`
	AttachmentsJSON       JSONSlice `gorm:"type:jsonb;not null;default:'[]'" json:"attachments_json"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type IDCOpsTicket struct {
	ID                    uint      `gorm:"primaryKey" json:"id"`
	UserID                uint      `gorm:"not null;index" json:"user_id"`
	Date                  time.Time `gorm:"type:date;not null;default:CURRENT_DATE;index" json:"date"`
	DutyPerson            string    `gorm:"size:100;not null" json:"duty_person"`
	IDCOpsTicketTypeID    *uint     `gorm:"index" json:"idc_ops_ticket_type_id"`
	VisitorOrganization   string    `gorm:"size:200;not null" json:"visitor_organization"`
	VisitorCount          int       `gorm:"not null;default:1" json:"visitor_count"`
	VisitorReason         string    `gorm:"type:text;not null" json:"visitor_reason"`
	CustomerServicePerson string    `gorm:"size:100" json:"customer_service_person"`
	ProcessingStatus      string    `gorm:"size:20;default:'pending';not null" json:"processing_status"`
	Remarks               string    `gorm:"type:text" json:"remarks"`
	AttachmentsJSON       JSONSlice `gorm:"type:jsonb;not null;default:'[]'" json:"attachments_json"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type FaultRecord struct {
	ID                    uint       `gorm:"primaryKey" json:"id"`
	UserID                uint       `gorm:"not null;index" json:"user_id"`
	Date                  time.Time  `gorm:"type:date;not null;default:CURRENT_DATE" json:"date"`
	DutyPerson            string     `gorm:"size:100;not null" json:"duty_person"`
	Status                string     `gorm:"size:20;default:'normal';not null" json:"status"`
	UserName              string     `gorm:"size:200;not null" json:"user_name"`
	ReceivedTime          time.Time  `gorm:"not null" json:"received_time"`
	FaultTypeID           uint       `gorm:"not null;index" json:"fault_type_id"`
	FaultSymptom          string     `gorm:"type:text;not null" json:"fault_symptom"`
	ProcessingProcess     string     `gorm:"type:text;not null" json:"processing_process"`
	CompletedTime         *time.Time `json:"completed_time"`
	CustomerServicePerson string     `gorm:"size:100" json:"customer_service_person"`
	ProcessingStatus      string     `gorm:"size:20;default:'pending';not null" json:"processing_status"`
	Remarks               string     `gorm:"type:text" json:"remarks"`
	AttachmentsJSON       JSONSlice  `gorm:"type:jsonb;not null;default:'[]'" json:"attachments_json"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

type DutyLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    *uint     `gorm:"index" json:"user_id"`
	Date      time.Time `gorm:"type:date;not null;index" json:"date"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Instruction struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Title     string    `gorm:"size:200;not null" json:"title"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TicketCategory struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:100;uniqueIndex;not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Ticket struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	UserID           uint      `gorm:"not null;index" json:"user_id"`
	TicketCategoryID *uint     `gorm:"index" json:"ticket_category_id"`
	Title            string    `gorm:"size:200;not null" json:"title"`
	Content          string    `gorm:"type:text;not null" json:"content"`
	Status           string    `gorm:"size:30;not null;default:'open'" json:"status"`
	Priority         string    `gorm:"size:30;not null;default:'medium'" json:"priority"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type TicketStatusHistory struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TicketID  uint      `gorm:"not null;index" json:"ticket_id"`
	Status    string    `gorm:"size:30;not null" json:"status"`
	ChangedBy *uint     `gorm:"index" json:"changed_by"`
	CreatedAt time.Time `json:"created_at"`
}

type SystemConfig struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Key         string    `gorm:"size:100;uniqueIndex;not null" json:"key"`
	Value       string    `gorm:"type:text" json:"value"`
	Description string    `gorm:"type:text" json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type AuditLog struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      *uint     `gorm:"index" json:"user_id"`
	Action      string    `gorm:"size:50;not null;index" json:"action"`
	TableName   string    `gorm:"size:50;index" json:"table_name"`
	RecordID    *uint     `gorm:"index" json:"record_id"`
	DetailsJSON JSONMap   `gorm:"type:jsonb;not null;default:'{}'" json:"details_json"`
	IPAddress   string    `gorm:"size:50" json:"ip_address"`
	CreatedAt   time.Time `json:"created_at"`
}

type BackupNotification struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	BackupFilePath string     `gorm:"size:300;not null" json:"backup_file_path"`
	BackupPassword string     `gorm:"size:200;not null" json:"backup_password"`
	RecipientEmail string     `gorm:"size:120;not null" json:"recipient_email"`
	SentAt         *time.Time `json:"sent_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

type Reminder struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	UserID           uint       `gorm:"not null;index" json:"user_id"`
	Title            string     `gorm:"size:200;not null" json:"title"`
	Content          string     `gorm:"type:text" json:"content"`
	StartDate        time.Time  `gorm:"type:date;not null" json:"start_date"`
	EndDate          time.Time  `gorm:"type:date;not null;index" json:"end_date"`
	RemindTime       string     `gorm:"size:5;not null;default:'09:00'" json:"remind_time"`
	RemindDaysBefore int        `gorm:"not null;default:2" json:"remind_days_before"`
	IsCompleted      bool       `gorm:"not null;default:false;index" json:"is_completed"`
	CompletedAt      *time.Time `json:"completed_at"`
	NotifiedAt       *time.Time `gorm:"index" json:"notified_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func AllModels() []any {
	return []any{
		&User{},
		&DutyLog{},
		&Instruction{},
		&TicketCategory{},
		&Ticket{},
		&TicketStatusHistory{},
		&WorkTicketType{},
		&FaultType{},
		&IDCOpsTicketType{},
		&TaskCategory{},
		&Attachment{},
		&WorkTicket{},
		&IDCOpsTicket{},
		&FaultRecord{},
		&AuditLog{},
		&SystemConfig{},
		&BackupNotification{},
		&Reminder{},
		&IdcDutyRecord{},
	}
}
