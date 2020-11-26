package sqlite

import (
	"context"
	"fmt"

	"github.com/jinzhu/gorm"
	otgorm "github.com/smacker/opentracing-gorm"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/label"
	_ "gorm.io/driver/sqlite"
)

type Conn struct {
	db *gorm.DB
}

func New() (*Conn, error) {
	db, err := gorm.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("Cannot create sql conn %w", err)
	}
	otgorm.AddGormCallbacks(db)
	err = db.Debug().CreateTable(&Users{}).Error
	if err != nil {
		return nil, fmt.Errorf("Cannot create required tables: %w", err)
	}
	return &Conn{db: db}, nil
}

type Users struct {
	ID  string `gorm:"id,omitempty"`
	Key string `gorm:"key,omitempty"`
}

func (c *Conn) FindUser(ctx context.Context, id string) (*Users, error) {
	user := new(Users)
	err := otgorm.SetSpanToGorm(ctx, c.db).Raw("SELECT * FROM users WHERE id = ?", id).First(user).Error
	if err != nil {
		span := trace.SpanFromContext(ctx)
		defer span.End(trace.WithRecord())
		span.RecordError(ctx, err)
		span.AddEvent(ctx, "error finding user", label.String("userId", id))
		return nil, fmt.Errorf("Cannot find user")
	}
	return user, nil
}
