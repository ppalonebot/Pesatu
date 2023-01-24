package images

import (
	"time"
)

type ImageMetadata struct {
	Filename   string    `json:"filename" bson:"filename"`
	Owner      string    `json:"owner" bson:"owner"`
	Contentype string    `json:"content_type" bson:"content_type"`
	UploadDate time.Time `json:"upload_date" bson:"upload_date"`
}
