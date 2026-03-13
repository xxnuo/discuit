package db

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/discuitnet/discuit/internal/uid"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type Driver string

const (
	DriverMariaDB    Driver = "mariadb"
	DriverPostgreSQL Driver = "postgresql"
	DriverSQLite     Driver = "sqlite3"
)

type UID uid.ID

func UIDFrom(id uid.ID) UID {
	return UID(id)
}

func (u UID) Raw() uid.ID {
	return uid.ID(u)
}

func (u UID) String() string {
	return uid.ID(u).String()
}

func (u UID) Value() (driver.Value, error) {
	id := uid.ID(u)
	if id.Zero() {
		return "", nil
	}
	return id.String(), nil
}

func (u *UID) Scan(src any) error {
	if src == nil {
		*u = UID{}
		return nil
	}

	var text string
	switch v := src.(type) {
	case string:
		text = v
	case []byte:
		text = string(v)
	default:
		return fmt.Errorf("scan uid: unsupported type %T", src)
	}

	text = strings.TrimSpace(text)
	if text == "" {
		*u = UID{}
		return nil
	}

	id, err := uid.FromString(text)
	if err != nil {
		return err
	}
	*u = UID(id)
	return nil
}

func (UID) GormDataType() string {
	return "uid"
}

func (UID) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	switch db.Dialector.Name() {
	case "postgres":
		return "char(24)"
	case "sqlite":
		return "text"
	default:
		return "char(24)"
	}
}

type IP net.IP

func IPFrom(ip net.IP) IP {
	return IP(ip)
}

func (ip IP) Raw() net.IP {
	return net.IP(ip)
}

func (ip IP) String() string {
	if len(ip) == 0 {
		return ""
	}
	return net.IP(ip).String()
}

func (ip IP) Value() (driver.Value, error) {
	if len(ip) == 0 {
		return nil, nil
	}
	return net.IP(ip).String(), nil
}

func (ip *IP) Scan(src any) error {
	if src == nil {
		*ip = nil
		return nil
	}

	var text string
	switch v := src.(type) {
	case string:
		text = v
	case []byte:
		text = string(v)
	default:
		return fmt.Errorf("scan ip: unsupported type %T", src)
	}

	text = strings.TrimSpace(text)
	if text == "" {
		*ip = nil
		return nil
	}

	parsed := net.ParseIP(text)
	if parsed == nil {
		return fmt.Errorf("scan ip: invalid value %q", text)
	}
	*ip = IP(parsed)
	return nil
}

func (IP) GormDataType() string {
	return "ip"
}

func (IP) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	switch db.Dialector.Name() {
	case "postgres":
		return "varchar(45)"
	case "sqlite":
		return "text"
	default:
		return "varchar(45)"
	}
}

type JSONMap map[string]any

func (m JSONMap) Value() (driver.Value, error) {
	if len(m) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (m *JSONMap) Scan(src any) error {
	if src == nil {
		*m = nil
		return nil
	}
	b, err := bytesFrom(src)
	if err != nil {
		return err
	}
	if len(b) == 0 {
		*m = nil
		return nil
	}
	var value JSONMap
	if err := json.Unmarshal(b, &value); err != nil {
		return err
	}
	*m = value
	return nil
}

func (JSONMap) GormDataType() string {
	return "json"
}

func (JSONMap) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	switch db.Dialector.Name() {
	case "postgres":
		return "jsonb"
	default:
		return "json"
	}
}

type StringList []string

func (l StringList) Value() (driver.Value, error) {
	if len(l) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(l)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (l *StringList) Scan(src any) error {
	if src == nil {
		*l = nil
		return nil
	}
	b, err := bytesFrom(src)
	if err != nil {
		return err
	}
	if len(b) == 0 {
		*l = nil
		return nil
	}
	var value StringList
	if err := json.Unmarshal(b, &value); err != nil {
		return err
	}
	*l = value
	return nil
}

func (StringList) GormDataType() string {
	return "json"
}

func (StringList) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	switch db.Dialector.Name() {
	case "postgres":
		return "jsonb"
	default:
		return "json"
	}
}

type UIDList []UID

func (l UIDList) Value() (driver.Value, error) {
	if len(l) == 0 {
		return nil, nil
	}
	encoded := make([]string, len(l))
	for i, id := range l {
		encoded[i] = id.String()
	}
	b, err := json.Marshal(encoded)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (l *UIDList) Scan(src any) error {
	if src == nil {
		*l = nil
		return nil
	}
	b, err := bytesFrom(src)
	if err != nil {
		return err
	}
	if len(b) == 0 {
		*l = nil
		return nil
	}
	var encoded []string
	if err := json.Unmarshal(b, &encoded); err != nil {
		return err
	}
	value := make(UIDList, len(encoded))
	for i, item := range encoded {
		id, err := uid.FromString(item)
		if err != nil {
			return err
		}
		value[i] = UIDFrom(id)
	}
	*l = value
	return nil
}

func (UIDList) GormDataType() string {
	return "json"
}

func (UIDList) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	switch db.Dialector.Name() {
	case "postgres":
		return "jsonb"
	default:
		return "json"
	}
}

type Bytes12 []byte

func (b Bytes12) Value() (driver.Value, error) {
	if len(b) == 0 {
		return nil, nil
	}
	if len(b) != 12 {
		return nil, errors.New("bytes12: invalid length")
	}
	return []byte(b), nil
}

func (b *Bytes12) Scan(src any) error {
	if src == nil {
		*b = nil
		return nil
	}
	value, err := bytesFrom(src)
	if err != nil {
		return err
	}
	if len(value) != 12 {
		return errors.New("bytes12: invalid length")
	}
	out := make([]byte, 12)
	copy(out, value)
	*b = out
	return nil
}

func (Bytes12) GormDataType() string {
	return "bytes12"
}

func (Bytes12) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	switch db.Dialector.Name() {
	case "postgres":
		return "bytea"
	case "sqlite":
		return "blob"
	default:
		return "varbinary(12)"
	}
}

type Bytes16 []byte

func (b Bytes16) Value() (driver.Value, error) {
	if len(b) == 0 {
		return nil, nil
	}
	if len(b) != 16 {
		return nil, errors.New("bytes16: invalid length")
	}
	return []byte(b), nil
}

func (b *Bytes16) Scan(src any) error {
	if src == nil {
		*b = nil
		return nil
	}
	value, err := bytesFrom(src)
	if err != nil {
		return err
	}
	if len(value) != 16 {
		return errors.New("bytes16: invalid length")
	}
	out := make([]byte, 16)
	copy(out, value)
	*b = out
	return nil
}

func (Bytes16) GormDataType() string {
	return "bytes16"
}

func (Bytes16) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	switch db.Dialector.Name() {
	case "postgres":
		return "bytea"
	case "sqlite":
		return "blob"
	default:
		return "varbinary(16)"
	}
}

func bytesFrom(src any) ([]byte, error) {
	switch v := src.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	default:
		return nil, fmt.Errorf("unsupported source type %T", src)
	}
}

type Timestamp struct {
	time.Time
}

func (Timestamp) GormDataType() string {
	return "timestamp"
}

func (Timestamp) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	switch db.Dialector.Name() {
	case "postgres":
		return "timestamp with time zone"
	default:
		return "datetime"
	}
}

func (t Timestamp) Value() (driver.Value, error) {
	if t.Time.IsZero() {
		return nil, nil
	}
	return t.Time, nil
}

func (t *Timestamp) Scan(src any) error {
	if src == nil {
		t.Time = time.Time{}
		return nil
	}
	switch v := src.(type) {
	case time.Time:
		t.Time = v
		return nil
	case []byte:
		return t.Scan(string(v))
	case string:
		parsed, err := time.Parse(time.RFC3339Nano, v)
		if err != nil {
			return err
		}
		t.Time = parsed
		return nil
	default:
		return fmt.Errorf("scan timestamp: unsupported type %T", src)
	}
}

type contextKey string

const migrationContextKey contextKey = "discuit-db-migration"

func WithMigrationContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, migrationContextKey, true)
}
