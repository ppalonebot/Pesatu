package utils

import (
	"encoding/json"
	"errors"
	"math/rand"
	"reflect"
	"regexp"
	"strconv"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
)

func GenerateRandomNumber() string {
	rand.Seed(time.Now().UnixNano())
	// Generate a random number between 100000 and 999999
	num := rand.Intn(900000) + 100000

	// Convert the number to a string
	numString := strconv.Itoa(num)

	return numString
}

func ToDoc(v interface{}) (doc *bson.D, err error) {
	data, err := bson.Marshal(v)
	if err != nil {
		return
	}

	err = bson.Unmarshal(data, &doc)
	return
}

func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func ToRawMessage(s interface{}) (json.RawMessage, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

func IsValidName(s string) (bool, error) {
	if len(s) == 0 {
		return false, errors.New("name can not empty")
	}

	if len(s) > 50 {
		return false, errors.New("name to long, max 50 characters")
	}

	return true, nil
}

func IsValidActivationCode(s string) (bool, error) {
	if len(s) == 0 {
		return false, errors.New("code can not empty")
	}

	if len(s) > 6 {
		return false, errors.New("code to long, max 6 characters")
	}

	return true, nil
}

func IsValidPassword(s string) (bool, error) {
	if len(s) == 0 {
		return false, errors.New("password to can not empty")
	}

	if len(s) < 6 {
		return false, errors.New("password to short")
	}

	return true, nil
}

func IsValidUsername(s string) (bool, error) {
	if len(s) == 0 {
		return false, errors.New("username to can not empty")
	}

	if len(s) < 2 {
		return false, errors.New("username to short")
	}

	if len(s) > 20 {
		return false, errors.New("username to long, max 20 characters")
	}

	match, err := regexp.MatchString(`^[a-z0-9][a-z0-9-_]*$`, s)
	if !match || err != nil {
		return false, errors.New("username can only have alphanumeric charater, '-', '_', and can't start with '-' and '_'")
	}

	return true, nil
}

func IsValidEmail(s string) bool {
	return govalidator.IsEmail(s)
}

func IsValidUid(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

func CopyStruct(src, dst interface{}) {
	srcVal := reflect.ValueOf(src).Elem()
	dstVal := reflect.ValueOf(dst).Elem()

	for i := 0; i < srcVal.NumField(); i++ {
		srcField := srcVal.Field(i)
		srcType := srcVal.Type().Field(i)

		// Check if the field exists in the destination struct
		if dstVal.FieldByName(srcType.Name).IsValid() {
			dstField := dstVal.FieldByName(srcType.Name)
			dstField.Set(srcField)
		}
	}
}
