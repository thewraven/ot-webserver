package sqlite

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jinzhu/gorm"
	"github.com/thewraven/otgorm"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/label"
	_ "gorm.io/driver/sqlite"
)

const componentName = "gorm"

type Conn struct {
	db *gorm.DB
}

func New() (*Conn, error) {
	db, err := gorm.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("Cannot create sql conn %w", err)
	}
	otgorm.RegisterCallbacks(db, otgorm.WithTracer(global.Tracer(componentName)))
	err = db.Debug().CreateTable(&Users{}).Error
	if err != nil {
		return nil, fmt.Errorf("Cannot create required tables: %w", err)
	}
	return &Conn{db: db}, nil
}

var users = []Users{
	{ID: "user1", Name: "John"},
	{ID: "user2", Name: "Jean"},
	{ID: "user3", Name: "Rose"},
	{ID: "user4", Name: "Denisse"},
}

func (c *Conn) FillDB(ctx context.Context) error {
	ctx, span := global.Tracer(componentName).Start(ctx, "Creating users in database")
	defer span.End()
	db := otgorm.WithContext(ctx, c.db)
	for _, u := range users {
		u.Key = uuid.New().String()
		err := db.Save(u).Error
		if err != nil {
			span.RecordError(ctx, err)
			return err
		}
	}
	return nil
}

type Users struct {
	ID   string `gorm:"id,omitempty",json:"id"`
	Name string `gorm:"name,omitempty",json:"name"`
	Key  string `gorm:"key,omitempty",json:"key"`
}

func (c *Conn) FindUser(ctx context.Context, id string) (*Users, error) {
	ctx, span := global.Tracer(componentName).Start(ctx, "Finding users")
	defer span.End()
	user := new(Users)
	err := otgorm.WithContext(ctx, c.db).Raw("SELECT * FROM users WHERE id = ?", id).First(user).Error
	if err != nil {
		span.RecordError(ctx, err)
		span.AddEvent(ctx, "error finding user", label.String("userId", id))
		return nil, fmt.Errorf("Cannot find user")
	}
	return user, nil
}
