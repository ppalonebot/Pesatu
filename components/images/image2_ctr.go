package images

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"pesatu/auth"
	"pesatu/components/user"
	"pesatu/utils"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
)

type ImageController struct {
	userService user.I_UserRepo
	service     I_ImageRepo
}

func NewImageController(userService user.I_UserRepo, service I_ImageRepo) ImageController {
	return ImageController{userService, service}
}

func (me *ImageController) UploadImage(owner string, file *multipart.FileHeader) (*ImageMetadata, error) {
	bad := utils.ValidateLinkOrJS(file.Header.Get("Content-Type"))
	if bad {
		return nil, fmt.Errorf("invalid content-type")
	}

	return me.service.SaveImage(owner, file)
}

func (me *ImageController) FindImage(imageID string) (*gridfs.DownloadStream, error) {
	ok := len(imageID) > 0
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
	ok := len(imageID) > 0
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

	// Retrieve uploaded image file
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "error retrieving image file: " + err.Error()})
		return
	}

	if c.Request.ContentLength > MAX_IMAGE_SIZE {
		Logger.V(2).Error(fmt.Errorf("image size exceeds maximum allowed"), "error while uploading image")
		c.JSON(http.StatusBadRequest, gin.H{"error": "image size exceeds maximum allowed, max 2MB"})
		return
	}

	if validuser.GetCmd() != "UpdateAvatar" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ilegal token"})
		return
	}

	user, err := me.userService.FindUserById(validuser.GetUID())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(user.Avatar) == 0 && validuser.GetCode() != "#empty" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ilegal token code"})
		return
	}

	if len(user.Avatar) != 0 && !strings.HasSuffix(user.Avatar, validuser.GetCode()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ilegal token code 2"})
		return
	}

	// Save image metadata to separate collection
	imageMetadata, err := me.UploadImage(validuser.GetUID(), file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error uploading image: " + err.Error()})
		return
	}

	if validuser.GetCmd() == "UpdateAvatar" {
		if len(user.Avatar) > 0 {
			parts := strings.Split(user.Avatar, "/")
			id := parts[len(parts)-1]
			err = me.service.DeleteImage(id)
			if err != nil {
				Logger.Error(err, "error while deleting image")
			}
		}

		user.Avatar = utils.CreateImageLink(imageMetadata.Filename)
		user.UpdatedAt = time.Now()
		_, err = me.userService.UpdateUser(user.Id, user)

		if err != nil {
			Logger.Error(err, "error while updeting user avatar")
		}
	}

	// Return image metadata as JSON to client
	c.JSON(http.StatusOK, imageMetadata)
}

func (me *ImageController) GetImageHandler(c *gin.Context) {
	// Retrieve image ID from request parameters
	imageID := c.Param("id")
	Logger.V(2).Info(fmt.Sprintf("get image %s", imageID))

	// Retrieve image from GridFS bucket
	downloadStream, err := me.FindImage(imageID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "image " + err.Error()})
		return
	}
	defer downloadStream.Close()

	// Retrieve metadata about the file
	file := downloadStream.GetFile()
	var metadata ImageMetadata
	err = bson.Unmarshal(file.Metadata, &metadata)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "image metadata unavailable"})
		return
	}

	// Set headers based on metadata
	c.Header("Content-Type", metadata.Contentype)
	c.Header("Content-Length", strconv.Itoa(int(file.Length)))

	// Send image as response to client
	_, err = io.Copy(c.Writer, downloadStream)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error sending image: " + err.Error()})
		return
	}
}
