package userprofile

import (
	"fmt"
	"net/http"
	"pesatu/auth"
	"pesatu/jsonrpc2"
	"pesatu/user"
	"pesatu/utils"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
)

type ProfileController struct {
	userService    user.I_UserRepo
	profileService I_ProfileRepo
	mongoCli       *mongo.Client
}

func NewProfileController(userService user.I_UserRepo, profileService I_ProfileRepo, mongoCli *mongo.Client) ProfileController {
	return ProfileController{userService, profileService, mongoCli}
}

func (me *ProfileController) UpdateMyProfile(validuser *auth.Claims, update *UpdateUserProfile) (*ResponseUserProfile, *jsonrpc2.RPCError, int) {
	Logger.V(2).Info(fmt.Sprintf("update my profile %s", update.UID))

	if validuser.GetUID() != update.UID {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: "user uid did not match"}, http.StatusOK
	}

	ok := utils.IsValidUid(update.UID)
	if !ok {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: "forbidden, invalid input owner"}, http.StatusOK
	}

	user, err := me.userService.FindUserById(validuser.GetUID())
	if err != nil {
		if strings.Contains(err.Error(), "exists") {
			return nil, &jsonrpc2.RPCError{Code: http.StatusNotFound, Message: err.Error()}, http.StatusOK
		}
		return nil, &jsonrpc2.RPCError{Code: http.StatusBadGateway, Message: err.Error()}, http.StatusOK
	}
	if !user.Reg.Registered {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: "user unregistered"}, http.StatusOK
	}
	if user.Reg.Code != validuser.GetCode() {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: "invalid jwt"}, http.StatusOK
	}

	profile, err := me.profileService.FindProfileByOwner(user.UID)
	if err != nil {
		if strings.Contains(err.Error(), "exists") {
			// return nil, &jsonrpc2.RPCError{Code: http.StatusNotFound, Message: err.Error()}, http.StatusOK
			profile = nil
		} else {
			return nil, &jsonrpc2.RPCError{Code: http.StatusBadGateway, Message: err.Error()}, http.StatusOK
		}
	}

	errres := make([]*jsonrpc2.InputFieldError, 0, 6)

	if user.Name != update.Name {
		_, err = utils.IsValidName(update.Name)
		if err != nil {
			errres = append(errres, &jsonrpc2.InputFieldError{Error: err.Error(), Field: "name"})
		}
	}

	update.Username = strings.ToLower(update.Username)
	if user.Username != update.Username {
		_, err = utils.IsValidUsername(update.Username)
		if err != nil {
			errres = append(errres, &jsonrpc2.InputFieldError{Error: err.Error(), Field: "username"})
		} else {
			exist, _ := me.userService.FindUserByUsername(update.Username)
			if exist != nil {
				errres = append(errres, &jsonrpc2.InputFieldError{Error: "username unavailable", Field: "username"})
			}
		}
	}

	update.Email = strings.ToLower(update.Email)
	if user.Email != update.Email {
		ok := utils.IsValidEmail(update.Email)
		if !ok {
			errres = append(errres, &jsonrpc2.InputFieldError{Error: "email invalid", Field: "email"})
		} else {
			exist, _ := me.userService.FindUserByEmail(update.Email)
			if exist != nil {
				errres = append(errres, &jsonrpc2.InputFieldError{Error: "email unavailabe", Field: "email"})
			}
		}
	}

	if profile != nil && (update.Status != profile.Status) {
		if len(update.Status) > 100 {
			errres = append(errres, &jsonrpc2.InputFieldError{Error: "input status too long, max 100 chars", Field: "status"})
		} else {
			injected := utils.ValidateLinkOrJS(update.Status)
			if injected {
				errres = append(errres, &jsonrpc2.InputFieldError{Error: "invalid input status", Field: "status"})
			}
		}
	}

	if profile != nil && (update.Bio != profile.Bio) {
		if len(update.Bio) > 500 {
			errres = append(errres, &jsonrpc2.InputFieldError{Error: "input status too long, max 500 chars", Field: "bio"})
		} else {
			injected := utils.ValidateLinkOrJS(update.Bio)
			if injected {
				errres = append(errres, &jsonrpc2.InputFieldError{Error: "invalid input bio", Field: "bio"})
			}
		}
	}

	if profile != nil && (update.PPic != profile.PPic) {
		if len(update.PPic) > 36 {
			errres = append(errres, &jsonrpc2.InputFieldError{Error: "invalid ppic name", Field: "ppic"})
		} else {
			ok = utils.IsAlphaNumericLowcase(update.PPic) || len(update.PPic) == 0
			if !ok {
				errres = append(errres, &jsonrpc2.InputFieldError{Error: "invalid input ppic", Field: "ppic"})
			} else {
				//delete old
				if len(profile.PPic) > 0 {
					// Create a new GridFS bucket
					db := me.mongoCli.Database("pesatu")
					gridfsBucket, err := gridfs.NewBucket(db)
					if err == nil {
						// Delete image from GridFS bucket
						_ = gridfsBucket.Delete(profile.PPic)
					}
				}
			}
		}
	}

	if len(errres) > 0 {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: "forbidden, invalid input", Params: errres}, http.StatusOK
	}

	if profile == nil {
		//create new profile
		var p CreateProfile
		utils.CopyStruct(update, &p)
		p.CreatedAt = time.Now()
		p.UpdatedAt = p.CreatedAt
		p.Owner = user.UID
		profile, err = me.profileService.CreateProfile(&p)
		if err != nil {
			Logger.Error(err, "internal error, while creating new profile")
			return nil, &jsonrpc2.RPCError{Code: http.StatusInternalServerError, Message: err.Error()}, http.StatusOK
		}
	} else {
		//update profile
		utils.CopyStruct(update, profile)
		profile.UpdatedAt = time.Now()
		profile, err = me.profileService.UpdateProfile(profile.Id, profile)
		if err != nil {
			Logger.Error(err, "internal error, while updating profile")
			return nil, &jsonrpc2.RPCError{Code: http.StatusInternalServerError, Message: err.Error()}, http.StatusOK
		}
	}

	utils.CopyStruct(update, user)
	if len(update.PPic) == 0 {
		user.Avatar = ""
	} else {
		user.Avatar = fmt.Sprintf("/image/%s", update.PPic)
	}
	user.UpdatedAt = time.Now()
	user, err = me.userService.UpdateUser(user.Id, user)
	if err != nil {
		Logger.Error(err, "internal error, while updating user")
		return nil, &jsonrpc2.RPCError{Code: http.StatusInternalServerError, Message: err.Error()}, http.StatusOK
	}

	var resUserProfile ResponseUserProfile
	utils.CopyStruct(user, &resUserProfile)
	utils.CopyStruct(profile, &resUserProfile)
	resUserProfile.IsRegistered = user.Reg.Registered

	Logger.V(2).Info("update profile success")
	return &resUserProfile, nil, http.StatusCreated
}

func (me *ProfileController) FindMyProfile(validuser *auth.Claims, owner string) (*ResponseUserProfile, *jsonrpc2.RPCError, int) {
	Logger.V(2).Info(fmt.Sprintf("find profile by owner %s", owner))

	if validuser.GetUID() != owner {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: "user uid did not match"}, http.StatusOK
	}

	ok := utils.IsValidUid(owner)
	if !ok {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: "invalid input owner"}, http.StatusOK
	}

	user, err := me.userService.FindUserById(validuser.GetUID())
	if err != nil {
		if strings.Contains(err.Error(), "exists") {
			return nil, &jsonrpc2.RPCError{Code: http.StatusNotFound, Message: err.Error()}, http.StatusOK
		}
		return nil, &jsonrpc2.RPCError{Code: http.StatusBadGateway, Message: err.Error()}, http.StatusOK
	}
	if !user.Reg.Registered {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: "user unregistered"}, http.StatusOK
	}
	if user.Reg.Code != validuser.GetCode() {
		return nil, &jsonrpc2.RPCError{Code: http.StatusForbidden, Message: "invalid jwt"}, http.StatusOK
	}

	profile, err := me.profileService.FindProfileByOwner(owner)
	if err != nil {
		if strings.Contains(err.Error(), "exists") {
			//return nil, &jsonrpc2.RPCError{Code: http.StatusNotFound, Message: err.Error()}, http.StatusOK
			profile = &DBProfile{}
		} else {
			return nil, &jsonrpc2.RPCError{Code: http.StatusBadGateway, Message: err.Error()}, http.StatusOK
		}
	}

	var resUserProfile ResponseUserProfile
	utils.CopyStruct(user, &resUserProfile)
	utils.CopyStruct(profile, &resUserProfile)
	resUserProfile.IsRegistered = user.Reg.Registered

	Logger.V(2).Info("found profile", user.Username)
	return &resUserProfile, nil, http.StatusOK
}
