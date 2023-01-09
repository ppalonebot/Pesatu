package images

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"pesatu/auth"
	"pesatu/utils"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
)

type ImageController struct {
	service I_ImageRepo
}

func NewImageController(service I_ImageRepo) ImageController {
	return ImageController{service}
}

func (me *ImageController) UploadImage(owner string, file *multipart.FileHeader) (*ImageMetadata, error) {
	bad := utils.ValidateLinkOrJS(file.Header.Get("Content-Type"))
	if bad {
		return nil, fmt.Errorf("invalid content-type")
	}

	return me.service.SaveImage(owner, file)
}

func (me *ImageController) FindImage(imageID string) (*gridfs.DownloadStream, error) {
	ok := len(imageID) == 0
	if !ok {
		return nil, fmt.Errorf("id can not empty")
	}

	ok = len(imageID) <= 36
	if !ok {
		return nil, fmt.Errorf("id is invalid")
	}

	ok = utils.IsAlphaNumericLowcase(imageID)
	if !ok {
		return nil, fmt.Errorf("not found")
	}

	return me.service.FindImage(imageID)
}

func (me *ImageController) DeleteImage(imageID string) error {
	ok := len(imageID) == 0
	if !ok {
		return fmt.Errorf("id can not empty")
	}

	ok = len(imageID) <= 36
	if !ok {
		return fmt.Errorf("id is invalid")
	}

	ok = utils.IsAlphaNumericLowcase(imageID)
	if !ok {
		return fmt.Errorf("not found")
	}

	return me.service.DeleteImage(imageID)
}

func (me *ImageController) ImageUploadHandler(c *gin.Context) {
	vuser, ok := c.Get("validuser")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	validuser := vuser.(*auth.Claims)
	if validuser.IsExpired() {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "session expired"})
		return
	}

	if c.Request.ContentLength > MAX_IMAGE_SIZE {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image size exceeds maximum allowed, max 5MB"})
		return
	}

	//todo! use special token to limit upload

	// Retrieve uploaded image file
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "error retrieving image file: " + err.Error()})
		return
	}

	// Save image metadata to separate collection
	imageMetadata, err := me.UploadImage(validuser.GetUID(),file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error uploading image: " + err.Error()})
		return
	}

	// Return image metadata as JSON to client
	c.JSON(http.StatusOK, imageMetadata)
}

func (me *ImageController) GetImageHandler(c *gin.Context) {
	// Retrieve image ID from request parameters
	imageID := c.Param("id")

	// Retrieve image from GridFS bucket
	downloadStream, err := me.FindImage(imageID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "image " + err.Error()})
		return
	}
	defer downloadStream.Close()

	// Retrieve metadata about the file
	file := downloadStream.GetFile()
	var metadata FileMetadata
	err = bson.Unmarshal(file.Metadata, &metadata)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "image metadata unavailable"})
		return
	}

	// Set headers based on metadata
	c.Header("Content-Type", metadata.ContentType)
	c.Header("Content-Length", strconv.Itoa(int(file.Length)))

	// Send image as response to client
	_, err = io.Copy(c.Writer, downloadStream)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error sending image: " + err.Error()})
		return
	}
}
