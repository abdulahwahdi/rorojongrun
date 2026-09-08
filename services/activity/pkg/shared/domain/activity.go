package domain

import (
	"go.mongodb.org/mongo-driver/v2/bson"
	"time"
)

// Activity model
type Activity struct {
	ID        bson.ObjectID `sql:"id" bson:"_id" json:"id"`
	Field     string        `sql:"field" bson:"field" json:"field"`
	CreatedAt time.Time     `sql:"created_at" bson:"created_at" json:"created_at"`
	UpdatedAt time.Time     `sql:"updated_at" bson:"updated_at" json:"updated_at"`
}

// CollectionName return collection name of Activity model
func (Activity) CollectionName() string {
	return "activities"
}
