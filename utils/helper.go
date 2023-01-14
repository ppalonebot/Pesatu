package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
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
	sTrimmed := strings.TrimSpace(s)
	if s != sTrimmed {
		return false, errors.New("name needs to be trimmed")
	}

	if len(s) == 0 {
		return false, errors.New("name can not empty")
	}

	if len(s) > 50 {
		return false, errors.New("name to long, max 50 characters")
	}

	injected := ValidateLinkOrJS(s)
	if injected {
		return false, errors.New("invalid name")
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
		return false, errors.New("password can not empty")
	}

	if len(s) < 6 {
		return false, errors.New("password to short")
	}

	return true, nil
}

func IsAlphaNumericLowcase(s string) bool {
	match, err := regexp.MatchString(`^[a-z0-9]*$`, s)
	if !match || err != nil {
		return false
	}

	return true
}

func IsValidUsername(s string) (bool, error) {
	if len(s) == 0 {
		return false, errors.New("username can not empty")
	}

	if len(s) < 2 {
		return false, errors.New("username to short")
	}

	if len(s) > 20 {
		return false, errors.New("username to long, max 20 characters")
	}

	injected := ValidateLinkOrJS(s)
	if injected {
		return false, errors.New("invalid username")
	}

	match, err := regexp.MatchString(`^[a-z0-9][a-z0-9-_]*$`, s)
	if !match || err != nil {
		return false, errors.New("username can only have alphanumeric,'-', '_', and start with alphanumeric characters")
	}

	return true, nil
}

func IsValidEmail(s string) bool {
	if len(s) < 3 {
		return false
	}
	return govalidator.IsEmail(s)
}

func IsValidUid(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

func IsValidDate(date string) (bool, error) {
	if len(date) == 0 {
		return false, errors.New("input date can not empty")
	}

	_, err := time.Parse("2006/01/02", date)
	return err == nil, err
}

func ValidateLinkOrJS(s string) bool {
	// Use the regexp package to compile a regular expression for matching links and JavaScript
	linkRegexp := regexp.MustCompile(`(http|ftp|https):\/\/([\w\-_]+(?:(?:\.[\w\-_]+)+))([\w\-\.,@?^=%&amp;:/~\+#]*[\w\-\@?^=%&amp;/~\+#])?`)
	jsRegexp := regexp.MustCompile(`(?:<script.*>)(\n|\r|.)*?(?:<\/script>)`)

	// Check if the string contains a link or JavaScript using the regular expression
	if linkRegexp.MatchString(s) || jsRegexp.MatchString(s) {
		return true
	}
	return false
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

func GetTemplateData(file_html string) (*template.Template, error) {
	templateData, err := template.ParseFiles(fmt.Sprintf("../template/%s", file_html))
	if err != nil {
		// Get the current working directory
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}

		// Get the absolute path of the current file
		absPath, err := filepath.Abs(wd)
		if err != nil {
			return nil, err
		}

		templateData, err = template.ParseFiles(fmt.Sprintf("%s/template/%s", absPath, file_html))
		if err != nil {
			return nil, err
		}
	}

	return templateData, nil
}

func CreateImageLink(fname string) string {
	return fmt.Sprintf("/image/%s", fname)
}
