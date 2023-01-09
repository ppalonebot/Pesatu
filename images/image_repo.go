package images

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"pesatu/utils"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type I_ImageRepo interface {
	SaveImage(owner string, file *multipart.FileHeader) (*ImageMetadata, error)
	FindImage(imageID string) (*gridfs.DownloadStream, error)
	DeleteImage(imageID string) error
}

type ImageService struct {
	gridfsBucket *gridfs.Bucket
	ctx          context.Context
}

func NewImageService(gridfsBucket *gridfs.Bucket, ctx context.Context) I_ImageRepo {
	return &ImageService{gridfsBucket, ctx}
}

func (me *ImageService) SaveImage(owner string, file *multipart.FileHeader) (*ImageMetadata, error) {
	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("error opening image file: %s", err.Error())
	}
	defer src.Close()

	// Generate a unique ID for the image
	imageID := primitive.NewObjectID()

	// Save image metadata to separate collection
	metadata := &ImageMetadata{
		Filename:   imageID.Hex(),
		Owner:      owner,
		Contentype: file.Header.Get("Content-Type"),
		UploadDate: time.Now(),
	}

	imageMetadata, err := utils.ToDoc(metadata)
	if err != nil {
		return nil, fmt.Errorf("error creating metadata: %s", err.Error())
	}

	// Save image to GridFS bucket using unique ID as filename
	uploadStream, err := me.gridfsBucket.OpenUploadStreamWithID(imageID, imageID.Hex(), options.GridFSUpload().SetMetadata(imageMetadata))
	if err != nil {
		return nil, fmt.Errorf("error saving image to GridFS bucket: %s", err.Error())
	}
	defer uploadStream.Close()

	_, err = io.Copy(uploadStream, src)
	if err != nil {
		return nil, fmt.Errorf("error copying image to GridFS bucket: %s", err.Error())
	}

	return metadata, nil
}

func (me *ImageService) FindImage(imageID string) (*gridfs.DownloadStream, error) {
	// Retrieve image from GridFS bucket
	downloadStream, err := me.gridfsBucket.OpenDownloadStreamByName(imageID)
	if err != nil {
		return nil, fmt.Errorf("image not found: %s", err.Error())
	}

	return downloadStream, nil
}

func (me *ImageService) DeleteImage(imageID string) error {
	err := me.gridfsBucket.Delete(imageID)
	if err != nil {
		return fmt.Errorf("image not found: %s", err.Error())
	}

	return nil
}
